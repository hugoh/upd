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
	"time"

	"github.com/google/shlex"
	"github.com/hugoh/upd/internal/logger"
)

// DownAction holds configuration for actions executed when connection is down.
type DownAction struct {
	After        time.Duration
	Every        time.Duration
	BackoffLimit time.Duration
	Exec         string
	StopExec     string
}

// DownActionIteration tracks the current iteration state for exponential backoff.
type DownActionIteration struct {
	iteration    int
	sleepTime    time.Duration
	limitReached bool
}

// DownActionLoop manages execution of down action commands.
type DownActionLoop struct {
	da         *DownAction
	it         *DownActionIteration
	cancelFunc context.CancelFunc
	//nolint:containedctx // loopCtx stored to bind StopExec commands on loop exit
	loopCtx    context.Context
	mu         sync.RWMutex
	currentCmd *exec.Cmd
	cmdMu      sync.Mutex
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
	// ErrInvalidCommand is returned when the command is invalid.
	ErrInvalidCommand = errors.New("invalid command")
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

	dal.mu.RLock()
	iteration := dal.it.iteration
	dal.mu.RUnlock()

	// #nosec G204 // Command is validated by shlex.Split() and validateCommand() before execution
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	var stderrBuf bytes.Buffer

	cmd.Stderr = &stderrBuf

	cmdEnv := os.Environ()
	cmdEnv = append(cmdEnv, fmt.Sprintf("UPD_ITERATION=%d", iteration))
	cmd.Env = cmdEnv
	logger.L.Info(
		"executing command",
		logger.LogComponent,
		logger.LogComponentDownAction,
		"exec",
		cmd.String(),
		"iteration",
		iteration,
	)

	if err = cmd.Start(); err != nil {
		logger.L.Error("failed to run",
			logger.LogComponent, logger.LogComponentDownAction, "exec", cmd.String(), "error", err)

		return fmt.Errorf("failed to execute DownAction: %w", err)
	}

	dal.cmdMu.Lock()
	dal.currentCmd = cmd
	dal.cmdMu.Unlock()

	go func() {
		waitErr := cmd.Wait()

		dal.cmdMu.Lock()
		if dal.currentCmd == cmd {
			dal.currentCmd = nil
		}
		dal.cmdMu.Unlock()

		if waitErr != nil {
			logger.L.Warn(
				"error executing command",
				logger.LogComponent,
				logger.LogComponentDownAction,
				"exec",
				cmd.String(),
				"error",
				waitErr,
				"stderr",
				string(bytes.TrimSpace(stderrBuf.Bytes())),
			)
		}
	}()

	return nil
}

// NewDownActionIteration creates a new iteration tracker.
func NewDownActionIteration() *DownActionIteration {
	return &DownActionIteration{
		iteration: -1,
	}
}

// NewDownActionLoop creates a new loop context for the down action.
func (da *DownAction) NewDownActionLoop(ctx context.Context) (*DownActionLoop, context.Context) {
	ctx, cancelFunc := context.WithCancel(ctx)
	dal := &DownActionLoop{
		da:         da,
		it:         NewDownActionIteration(),
		cancelFunc: cancelFunc,
		loopCtx:    ctx,
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

func (dal *DownActionLoop) iterate() {
	dal.mu.Lock()
	defer dal.mu.Unlock()

	dal.it.iteration++
	switch dal.it.iteration {
	case 0:
		dal.it.sleepTime = dal.da.After
	case 1:
		dal.it.sleepTime = dal.da.Every
	default:
		if !dal.it.limitReached {
			dal.it.sleepTime = time.Duration(BackoffFactor * float64(dal.it.sleepTime))
			if dal.da.BackoffLimit != 0 && dal.it.sleepTime >= dal.da.BackoffLimit {
				dal.it.sleepTime = dal.da.BackoffLimit
				dal.it.limitReached = true
			}
		}
	}

	logger.L.Debug("iteration details",
		logger.LogComponent, logger.LogComponentDownAction, "iteration", dal.it.iteration,
		"sleepTime", dal.it.sleepTime,
		"limitReached", dal.it.limitReached)
}

func (dal *DownActionLoop) run(ctx context.Context) {
	logger.L.Debug("down action loop started",
		logger.LogComponent, logger.LogComponentDownAction)
	dal.iterate()

	for {
		dal.mu.RLock()
		sleepTime := dal.it.sleepTime
		dal.mu.RUnlock()

		logger.L.Debug(
			"sleeping",
			logger.LogComponent,
			logger.LogComponentDownAction,
			"duration",
			sleepTime,
		)
		timer := time.NewTimer(sleepTime)

		select {
		case <-ctx.Done():
			timer.Stop()
			logger.L.Debug("canceled", logger.LogComponent, logger.LogComponentDownAction)

			return
		case <-timer.C:
			timer.Stop()
		}

		dal.killCurrentCmd()

		err := dal.Execute(ctx, dal.da.Exec)
		if err != nil {
			logger.L.Error(
				"failed to execute",
				logger.LogComponent,
				logger.LogComponentDownAction,
				"iteration",
				dal.it.iteration,
				"error",
				err,
			)
		} else {
			logger.L.Debug("command succeeded",
				logger.LogComponent, logger.LogComponentDownAction)
		}

		if dal.da.Every > 0 {
			dal.iterate()
		} else {
			logger.L.Debug("down action loop complete",
				logger.LogComponent, logger.LogComponentDownAction)
			break
		}
	}
}
