package logic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/google/shlex"
	"github.com/hugoh/upd/internal/logger"
	"github.com/sirupsen/logrus"
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
	// #nosec G204 // Command is validated by shlex.Split() and validateCommand() before execution
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmdEnv := os.Environ()
	cmdEnv = append(cmdEnv, fmt.Sprintf("UPD_ITERATION=%d", dal.it.iteration))
	cmd.Env = cmdEnv
	logger.L.WithField("exec", cmd.String()).Info("[DownAction] executing command")
	err = cmd.Start()
	if err != nil {
		logger.L.WithField("exec", cmd.String()).WithError(err).Error("[DownAction] failed to run")

		return fmt.Errorf("failed to execute DownAction: %w", err)
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.L.WithField("exec", cmd.String()).WithError(err).Warn("[DownAction] error executing command")
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
	}

	return dal, ctx
}

// Start begins the down action loop in a goroutine.
func (da *DownAction) Start(ctx context.Context) *DownActionLoop {
	dal, ctx := da.NewDownActionLoop(ctx)
	logger.L.Debug("[DownAction] kicking off run loop")
	go dal.run(ctx)

	return dal
}

// Stop cancels the down action loop and executes the stop command.
func (dal *DownActionLoop) Stop(ctx context.Context) {
	if dal.da.StopExec != "" {
		err := dal.Execute(ctx, dal.da.StopExec)
		if err != nil && !errors.Is(err, ErrNoCommand) {
			logger.L.WithError(err).Warn("[DownAction] failed to execute stop command")
		}
	}
	logger.L.Debug("[DownAction] sending shutdown signal")
	dal.cancelFunc()
}

func (dal *DownActionLoop) iterate() {
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
	logger.L.WithFields(logrus.Fields{
		"iteration":    dal.it.iteration,
		"sleepTime":    dal.it.sleepTime,
		"limitReached": dal.it.limitReached,
	}).Trace("[DownAction] iteration details")
}

func (dal *DownActionLoop) run(ctx context.Context) {
	dal.iterate()
	for {
		select {
		case <-ctx.Done():
			logger.L.Debug("[DownAction] canceled")

			return
		case <-time.After(dal.it.sleepTime):
		}
		err := dal.Execute(ctx, dal.da.Exec)
		if err != nil {
			logger.L.WithError(err).Error("[DownAction] failed to execute")
		}
		if dal.da.Every > 0 {
			dal.iterate()
		} else {
			break
		}
	}
}
