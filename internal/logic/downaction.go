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
	da         *DownAction
	cancelFunc context.CancelFunc
	//nolint:containedctx // loopCtx stored to bind StopExec commands on loop exit
	loopCtx      context.Context
	iteration    atomic.Int64
	sleepTime    time.Duration
	limitReached bool
	currentCmd   *exec.Cmd
	cmdMu        sync.Mutex
}

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
	if execString == "" {
		return ErrNoCommand
	}

	command, errSh := shlex.Split(execString)
	if errSh != nil {
		return fmt.Errorf("failed to parse DownAction definition: %w", errSh)
	}

	err := validateCommand(command)
	if err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	iteration := dal.iteration.Load()

	// #nosec G204 // Command is validated by shlex.Split() and validateCommand() before execution
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	var stderrBuf bytes.Buffer

	cmd.Stderr = &stderrBuf

	cmdEnv := os.Environ()
	cmdEnv = append(cmdEnv, fmt.Sprintf("UPD_ITERATION=%d", iteration))
	cmd.Env = cmdEnv
	logger.L.Info(
		"executing command",
		logger.LogComponent, logger.LogComponentDownAction,
		"exec", cmd.String(),
		"iteration", iteration,
	)

	if err = cmd.Start(); err != nil {
		logger.L.Error("failed to run",
			logger.LogComponent, logger.LogComponentDownAction, "exec", cmd.String(), "error", err)

		return fmt.Errorf("failed to execute DownAction: %w", err)
	}

	dal.cmdMu.Lock()
	dal.currentCmd = cmd
	dal.cmdMu.Unlock()

	go dal.waitForCmd(cmd, &stderrBuf)

	return nil
}

// NewDownActionLoop creates a new loop context for the down action.
func (da *DownAction) NewDownActionLoop(ctx context.Context) (*DownActionLoop, context.Context) {
	ctx, cancelFunc := context.WithCancel(ctx)
	dal := &DownActionLoop{
		da:         da,
		cancelFunc: cancelFunc,
		loopCtx:    ctx,
		sleepTime:  da.After,
	}

	return dal, ctx
}

// Start begins the down action loop in a goroutine.
func (da *DownAction) Start(ctx context.Context) *DownActionLoop {
	dal, ctx := da.NewDownActionLoop(ctx)

	logger.L.Debug("kicking off run loop", logger.LogComponent, logger.LogComponentDownAction)

	go dal.run(ctx)

	return dal
}

// Stop cancels the down action loop and executes the stop command.
// The stop command is bound to loopCtx so cancelFunc kills it on loop exit.
func (dal *DownActionLoop) Stop(_ context.Context) {
	dal.killCurrentCmd()

	if dal.da.StopExec != "" {
		//nolint:contextcheck // loopCtx derived from the same hierarchy as ctx
		err := dal.Execute(dal.loopCtx, dal.da.StopExec)
		if err != nil && !errors.Is(err, ErrNoCommand) {
			logger.L.Warn(
				"failed to execute stop command",
				logger.LogComponent,
				logger.LogComponentDownAction,
				"error",
				err,
			)
		}
	}

	logger.L.Debug("sending shutdown signal", logger.LogComponent, logger.LogComponentDownAction)
	dal.cancelFunc()
}

// Status returns a snapshot of the current down action loop state.
func (dal *DownActionLoop) Status() status.DownActionStatus {
	return status.DownActionStatus{
		Iteration:     dal.iteration.Load(),
		SleepTime:     status.ReadableDuration(dal.sleepTime),
		BackoffCapped: dal.limitReached,
	}
}

func (dal *DownActionLoop) waitForCmd(cmd *exec.Cmd, stderrBuf *bytes.Buffer) {
	waitErr := cmd.Wait()

	dal.cmdMu.Lock()
	if dal.currentCmd == cmd {
		dal.currentCmd = nil
	}
	dal.cmdMu.Unlock()

	if waitErr != nil {
		logger.L.Warn(
			"error executing command",
			logger.LogComponent, logger.LogComponentDownAction,
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

	logger.L.Warn("killing current command",
		logger.LogComponent, logger.LogComponentDownAction, "pid", dal.currentCmd.Process.Pid)

	if dal.currentCmd.Process != nil {
		if err := dal.currentCmd.Process.Kill(); err != nil {
			logger.L.Warn(
				"failed to kill current command",
				logger.LogComponent,
				logger.LogComponentDownAction,
				"error",
				err,
			)
		}
	}

	dal.currentCmd = nil
}

func (dal *DownActionLoop) nextSleep() time.Duration {
	dal.iteration.Add(1)

	switch dal.iteration.Load() {
	case 1:
		dal.sleepTime = dal.da.Every
	default:
		if !dal.limitReached {
			next := time.Duration(BackoffFactor * float64(dal.sleepTime))
			if dal.da.BackoffLimit != 0 && next >= dal.da.BackoffLimit {
				next = dal.da.BackoffLimit
				dal.limitReached = true
			}

			dal.sleepTime = next
		}
	}

	logger.L.Debug("iteration details",
		logger.LogComponent, logger.LogComponentDownAction, "iteration", dal.iteration.Load(),
		"sleepTime", dal.sleepTime,
		"limitReached", dal.limitReached)

	return dal.sleepTime
}

func (dal *DownActionLoop) run(ctx context.Context) {
	logger.L.Debug("down action loop started",
		logger.LogComponent, logger.LogComponentDownAction)

	for {
		logger.L.Debug(
			"sleeping",
			logger.LogComponent,
			logger.LogComponentDownAction,
			"duration",
			dal.sleepTime,
		)
		timer := time.NewTimer(dal.sleepTime)

		select {
		case <-ctx.Done():
			timer.Stop()
			logger.L.Debug("canceled", logger.LogComponent, logger.LogComponentDownAction)

			return
		case <-timer.C:
		}

		dal.killCurrentCmd()

		err := dal.Execute(ctx, dal.da.Exec)
		if err != nil {
			logger.L.Error(
				"failed to execute",
				logger.LogComponent,
				logger.LogComponentDownAction,
				"iteration",
				dal.iteration.Load(),
				"error",
				err,
			)
		} else {
			logger.L.Debug("command succeeded",
				logger.LogComponent, logger.LogComponentDownAction)
		}

		if dal.da.Every > 0 {
			dal.sleepTime = dal.nextSleep()
		} else {
			logger.L.Debug("down action loop complete",
				logger.LogComponent, logger.LogComponentDownAction)

			break
		}
	}
}
