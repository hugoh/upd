// Package logic provides the core monitoring loop and down action handling.
package logic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/shlex"
	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/status"
)

// DownAction holds configuration for actions executed when connection is down.
type DownAction struct {
	After        time.Duration
	Every        time.Duration
	BackoffLimit time.Duration
	Exec         string
	StopExec     string
}

// DownActionLoop manages execution of down action commands.
type DownActionLoop struct {
	da           *DownAction
	cancelFunc   context.CancelFunc
	iteration    atomic.Uint32
	sleepTime    atomic.Int64
	limitReached atomic.Bool
	currentCmd   *exec.Cmd
	cmdMu        sync.Mutex
	done         chan struct{}
}

// StopExecTimeout bounds how long the stop command may run.
const StopExecTimeout = 30 * time.Second

// BackoffFactor is the multiplier for exponential backoff.
// Each iteration beyond the second will increase the delay by this factor.
// Value of 1.5 results in: 10s -> 15s -> 22.5s -> 33.75s...
const BackoffFactor = 1.5

var (
	// ErrNoCommand is returned when no command is provided for execution.
	ErrNoCommand = errors.New("no command to execute")
	// ErrEmptyCommand is returned when the command name is empty.
	ErrEmptyCommand = errors.New("command name cannot be empty")
)

func validateCommand(command []string) error {
	if len(command) == 0 {
		return ErrNoCommand
	}

	if command[0] == "" {
		return ErrEmptyCommand
	}

	return nil
}

// Execute runs the specified command string with the iteration context.
func (dal *DownActionLoop) Execute(ctx context.Context, execString string) error {
	cmd, stderrBuf, err := dal.startCommand(ctx, execString)
	if err != nil {
		return err
	}

	dal.cmdMu.Lock()
	dal.currentCmd = cmd
	dal.cmdMu.Unlock()

	go dal.waitForCmd(cmd, stderrBuf)

	return nil
}

// NewDownActionLoop creates a new loop context for the down action. done
// starts closed since no run loop is running yet; Stop() must not block
// waiting for a run loop that Start() never launched.
func (da *DownAction) NewDownActionLoop(ctx context.Context) (*DownActionLoop, context.Context) {
	ctx, cancelFunc := context.WithCancel(ctx)
	done := make(chan struct{})
	close(done)

	dal := &DownActionLoop{
		da:         da,
		cancelFunc: cancelFunc,
		done:       done,
	}
	dal.sleepTime.Store(int64(da.After))

	return dal, ctx
}

// Start begins the down action loop in a goroutine.
func (da *DownAction) Start(ctx context.Context) *DownActionLoop {
	dal, ctx := da.NewDownActionLoop(ctx)
	dal.done = make(chan struct{})

	logger.DownAction().Debug("kicking off run loop")

	go dal.run(ctx)

	return dal
}

// Stop cancels the down action loop, waits for the run loop goroutine to
// exit so it cannot start a new command after Stop begins cleanup, kills any
// running command, and runs the stop command to completion. The stop command
// gets its own timeout-bounded context so loop cancellation cannot kill it.
func (dal *DownActionLoop) Stop(_ context.Context) {
	logger.DownAction().Debug("sending shutdown signal")
	dal.cancelFunc()
	<-dal.done
	dal.killCurrentCmd()

	if dal.da.StopExec != "" {
		//nolint:contextcheck // intentionally detached: must survive loop cancellation
		dal.runStopExec()
	}
}

// Status returns a snapshot of the current down action loop state.
func (dal *DownActionLoop) Status() status.DownActionStatus {
	return status.DownActionStatus{
		Iteration:     dal.iteration.Load(),
		SleepTime:     status.ReadableDuration(time.Duration(dal.sleepTime.Load())),
		BackoffCapped: dal.limitReached.Load(),
	}
}

// startCommand parses, validates, and starts the given command string.
func (dal *DownActionLoop) startCommand(
	ctx context.Context,
	execString string,
) (*exec.Cmd, *bytes.Buffer, error) {
	if execString == "" {
		return nil, nil, ErrNoCommand
	}

	command, errSh := shlex.Split(execString)
	if errSh != nil {
		return nil, nil, fmt.Errorf("failed to parse DownAction definition: %w", errSh)
	}

	err := validateCommand(command)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid command: %w", err)
	}

	iteration := dal.iteration.Load()

	// #nosec G204 // Command is validated by shlex.Split() and validateCommand() before execution
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	var stderrBuf bytes.Buffer

	cmd.Stderr = &stderrBuf

	cmd.Env = append(os.Environ(), fmt.Sprintf("UPD_ITERATION=%d", iteration))
	logger.DownAction().Info("executing command",
		"exec", cmd.String(),
		"iteration", iteration,
	)

	if err = cmd.Start(); err != nil {
		logger.DownAction().Error("failed to run",
			"exec", cmd.String(), "error", err)

		return nil, nil, fmt.Errorf("failed to execute DownAction: %w", err)
	}

	return cmd, &stderrBuf, nil
}

// runStopExec runs the stop command to completion on a context detached from
// the loop so cancellation cannot kill it, bounded by StopExecTimeout.
func (dal *DownActionLoop) runStopExec() {
	ctx, cancel := context.WithTimeout(context.Background(), StopExecTimeout)
	defer cancel()

	cmd, stderrBuf, err := dal.startCommand(ctx, dal.da.StopExec)
	if err != nil {
		logger.DownAction().Warn("failed to execute stop command", "error", err)

		return
	}

	dal.waitForCmd(cmd, stderrBuf)
}

func (dal *DownActionLoop) waitForCmd(cmd *exec.Cmd, stderrBuf *bytes.Buffer) {
	waitErr := cmd.Wait()

	dal.cmdMu.Lock()
	if dal.currentCmd == cmd {
		dal.currentCmd = nil
	}
	dal.cmdMu.Unlock()

	if waitErr != nil {
		logger.DownAction().Warn("error executing command",
			"exec", cmd.String(),
			"error", waitErr,
			"stderr", string(bytes.TrimSpace(stderrBuf.Bytes())),
		)
	}
}

// killCurrentCmd kills any currently running command and clears the reference.
func (dal *DownActionLoop) killCurrentCmd() {
	dal.cmdMu.Lock()
	defer dal.cmdMu.Unlock()

	if dal.currentCmd == nil {
		return
	}

	// currentCmd is only set after a successful Start, so Process is non-nil.
	logger.DownAction().Warn("killing current command",
		"pid", dal.currentCmd.Process.Pid)

	if err := dal.currentCmd.Process.Kill(); err != nil {
		logger.DownAction().Warn("failed to kill current command", "error", err)
	}

	dal.currentCmd = nil
}

func (dal *DownActionLoop) nextSleep() time.Duration {
	dal.iteration.Add(1)

	switch dal.iteration.Load() {
	case 1:
		dal.sleepTime.Store(int64(dal.da.Every))
	default:
		if !dal.limitReached.Load() {
			next := time.Duration(BackoffFactor * float64(dal.sleepTime.Load()))
			if dal.da.BackoffLimit != 0 && next >= dal.da.BackoffLimit {
				next = dal.da.BackoffLimit
				dal.limitReached.Store(true)
			}

			dal.sleepTime.Store(int64(next))
		}
	}

	sleepTime := time.Duration(dal.sleepTime.Load())
	logger.DownAction().Debug("iteration details",
		"iteration", dal.iteration.Load(),
		"sleepTime", sleepTime,
		"limitReached", dal.limitReached.Load())

	return sleepTime
}

func (dal *DownActionLoop) run(ctx context.Context) {
	defer close(dal.done)

	logger.DownAction().Debug("down action loop started")

	for {
		logger.DownAction().Debug("sleeping", "duration", time.Duration(dal.sleepTime.Load()))

		select {
		case <-ctx.Done():
			logger.DownAction().Debug("canceled")

			return
		case <-time.After(time.Duration(dal.sleepTime.Load())):
		}

		// select can race with cancellation: the timer case may be chosen
		// even though ctx was just canceled. Re-check explicitly so Stop's
		// <-dal.done wait is a reliable guarantee that no new command will
		// be started after cancellation.
		if ctx.Err() != nil {
			logger.DownAction().Debug("canceled")

			return
		}

		dal.killCurrentCmd()

		err := dal.Execute(ctx, dal.da.Exec)
		if err != nil {
			logger.DownAction().Error("failed to execute",
				"iteration",
				dal.iteration.Load(),
				"error",
				err,
			)
		} else {
			logger.DownAction().Debug("command succeeded")
		}

		if dal.da.Every > 0 {
			dal.nextSleep()
		} else {
			logger.DownAction().Debug("down action loop complete")

			break
		}
	}
}
