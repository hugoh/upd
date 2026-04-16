// Package logic provides the core monitoring loop and down action handling
// for the upd application.
//
// The Loop:
//
// The Loop struct manages the periodic execution of network connectivity
// checks. It runs continuously, executing checks at configurable intervals:
//   - Normal interval: Used when connection is up
//   - Down interval: Used when connection is down (typically more frequent)
//
// The loop uses a Delays map with boolean keys:
//   - true: Normal interval (connection up)
//   - false: Down interval (connection down)
//
// Example - Creating and running a loop:
//
//	loop := logic.NewLoop()
//	checks := &pkg.CheckList{
//		Ordered: pkg.Checks{
//			{
//				Probe:   pkg.NewHTTPProbe("https://example.com"),
//				Timeout: 10 * time.Second,
//			},
//		},
//	}
//	delays := logic.Delays{
//		true:  2 * time.Minute,  // Check every 2 minutes when up
//		false: 30 * time.Second,  // Check every 30 seconds when down
//	}
//	loop.Configure(checks, delays, nil, 24*time.Hour, nil)
//	go loop.Run(ctx)
//
// Down Actions:
//
// When the connection goes down, the loop can execute commands repeatedly:
//   - After: Initial delay before first execution
//   - Every: Interval between executions
//   - BackoffLimit: Maximum delay before exponential backoff stops
//   - Exec: Command to execute
//   - StopExec: Command to execute when connection comes back
//
// Example - Configure down action:
//
//	downAction := &logic.DownAction{
//		After:        60 * time.Second,  // Wait 60 seconds after connection drops
//		Every:        5 * time.Minute,    // Execute every 5 minutes
//		BackoffLimit: 30 * time.Minute,   // Max backoff of 30 minutes
//		Exec:         "/usr/local/bin/notify-down",
//		StopExec:     "/usr/local/bin/notify-up",
//	}
//	loop.Configure(checks, delays, downAction, 24*time.Hour, nil)
//
// Configuration:
//
// The Configure method initializes the loop with:
//   - checkList: List of probes to execute
//   - delays: Check intervals for up/down states
//   - downAction: Optional down action configuration
//   - retention: How long to keep status history (for statistics)
//   - statServerConfig: Optional HTTP statistics server configuration
//
// Context and Cancellation:
//
// The loop respects context cancellation. When the context is canceled,
// the loop will gracefully:
//   - Stop running checks
//   - Stop any down action execution
//   - Stop the statistics server
//   - Return from Run()
//
// Example - Graceful shutdown:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	go loop.Run(ctx)
//
//	// Later, to shutdown:
//	cancel()  // Context cancelled
//	loop.Stop(ctx)  // Wait for cleanup
package logic

import (
	"context"
	"errors"
	"time"

	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/status"
	"github.com/hugoh/upd/pkg"
)

// Delays maps connection state to check interval durations.
type Delays map[bool]time.Duration

// Loop manages periodic network connectivity checks.
type Loop struct {
	checkList        *pkg.CheckList
	delays           Delays
	downAction       *DownAction
	downActionLoop   *DownActionLoop
	statServer       *status.StatServer
	statServerConfig *status.StatServerConfig
	status           *status.Status
}

// NewLoop creates a new monitoring loop.
func NewLoop() *Loop {
	return &Loop{
		status: status.NewStatus(),
	}
}

// Configure initializes the loop with checks, delays, and optional down action.
func (l *Loop) Configure(
	checkList *pkg.CheckList,
	delays Delays,
	downAction *DownAction,
	retention time.Duration,
	statServerConfig *status.StatServerConfig,
) {
	l.checkList = checkList
	l.delays = delays
	l.downAction = downAction
	l.status.SetRetention(retention)
	l.statServerConfig = statServerConfig
}

// ErrDownActionRunning is returned when trying to start a down action while one is active.
var ErrDownActionRunning = errors.New("cannot start new DownAction when one is already running")

// DownActionStart initiates the down action execution loop.
func (l *Loop) DownActionStart(ctx context.Context) error {
	if l.downActionLoop != nil {
		return ErrDownActionRunning
	}

	l.downActionLoop = l.downAction.Start(ctx)

	return nil
}

// DownActionStop halts the current down action loop.
func (l *Loop) DownActionStop(ctx context.Context) {
	if l.downActionLoop == nil {
		// Nothing to stop
		return
	}

	l.downActionLoop.Stop(ctx)
	l.downActionLoop = nil
}

// ProcessCheck handles state changes and triggers down actions.
func (l *Loop) ProcessCheck(ctx context.Context, upStatus bool) {
	changed := l.status.Update(upStatus)
	if !changed {
		return
	}

	logger.L.Info("[Loop] connection status changed", "up", l.status.Up)

	if !l.hasDownAction() {
		return
	}

	if upStatus {
		l.DownActionStop(ctx)
	} else {
		err := l.DownActionStart(ctx)
		if err != nil {
			logger.L.Error("[Loop] could not start DownAction", "error", err)
		}
	}
}

// Run starts the monitoring loop.
func (l *Loop) Run(ctx context.Context) {
	var checker Checker

	if l.statServer == nil {
		l.statServer = status.StartStatServer(l.status, l.statServerConfig)
	}

	for {
		checkStatus, err := pkg.CheckerRun(ctx, checker, l.checkList.GetIterator())
		if err == nil {
			l.ProcessCheck(ctx, checkStatus)
		} else {
			logger.L.Error("[Loop] error", "error", err)
		}

		sleepTime := l.delays[l.status.Up]
		logger.L.Debug("[Loop] waiting for next loop iteration", "wait", sleepTime)

		timer := time.NewTimer(sleepTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			logger.L.Debug("[Loop] context canceled during sleep, exiting Run()")

			return
		case <-timer.C:
		}

		timer.Stop()
	}
}

// Stop gracefully shuts down the loop and its components.
func (l *Loop) Stop(ctx context.Context) {
	l.DownActionStop(ctx)

	if l.statServer != nil {
		l.statServer.StopStatServer(ctx)
	}
}

func (l *Loop) hasDownAction() bool {
	return l.downAction != nil
}

// Checker implements pkg.Checker for logging check lifecycle events.
type Checker struct{}

// CheckRun logs the start of a check.
func (checker Checker) CheckRun(chk pkg.Check) {
	probe := *chk.Probe
	logger.L.Debug(
		"[Check] running",
		"probe",
		probe,
		"protocol",
		probe.Scheme(),
		"timeout",
		chk.Timeout,
	)
}

// ProbeSuccess logs successful probe results.
func (checker Checker) ProbeSuccess(report *pkg.Report) {
	logger.L.Debug("[Check] success", report.LogAttrs()...)
}

// ProbeFailure logs failed probe results.
func (checker Checker) ProbeFailure(report *pkg.Report) {
	logger.L.Warn("[Check] failed", report.LogAttrs()...)
}
