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
// The loop uses a Delays struct:
//   - Up: Normal interval (connection up)
//   - Down: Down interval (connection down)
//
// Example - Creating and running a loop:
//
//	loop := logic.NewLoop()
//	checks := &check.List{
//		Ordered: check.Checks{
//			{
//				Probe:   check.NewHTTPProbe("https://example.com"),
//				Timeout: 10 * time.Second,
//			},
//		},
//	}
//	delays := logic.Delays{
//		Up:   2 * time.Minute,  // Check every 2 minutes when up
//		Down: 30 * time.Second, // Check every 30 seconds when down
//	}
//	loop.Configure(checks, delays, nil, status.BucketConfig{}, time.Minute, 5*time.Minute)
//	go loop.Run(ctx, nil)
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
//	loop.Configure(checks, delays, downAction, status.BucketConfig{}, time.Minute, 5*time.Minute)
//
// Configuration:
//
// The Configure method initializes the loop with:
//   - checkList: List of probes to execute
//   - delays: Check intervals for up/down states
//   - downAction: Optional down action configuration
//   - buckets: Probe-stat bucket granularity tuning (zero value for defaults)
//   - periods: Report periods for statistics (retention is derived from the max period)
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

	"github.com/hugoh/upd/internal/check"
	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/status"
)

// Delays holds the check interval durations for up and down states.
type Delays struct {
	Up   time.Duration
	Down time.Duration
}

// ForStatus returns the check interval for the given connection state.
func (d Delays) ForStatus(up bool) time.Duration {
	if up {
		return d.Up
	}

	return d.Down
}

// Loop manages periodic network connectivity checks.
type Loop struct {
	checkList      *check.List
	delays         Delays
	downAction     *DownAction
	downActionLoop *DownActionLoop
	statServer     *status.StatServer
	status         *status.Status
	rollingTracker *status.RollingProbeTracker
	lastSuccess    time.Time
	nextCheckAt    time.Time
}

// NewLoop creates a new monitoring loop.
func NewLoop() *Loop {
	return &Loop{
		status: status.NewStatus(),
	}
}

// Configure initializes the loop with checks, delays, and optional down action.
func (l *Loop) Configure(
	checkList *check.List,
	delays Delays,
	downAction *DownAction,
	buckets status.BucketConfig,
	periods ...time.Duration,
) {
	l.checkList = checkList
	l.delays = delays
	l.downAction = downAction

	var retention time.Duration
	for _, p := range periods {
		if p > retention {
			retention = p
		}
	}

	l.status.SetRetention(retention)

	if retention > 0 {
		l.rollingTracker = status.NewRollingProbeTracker(periods, buckets)
		l.status.SetRollingTracker(l.rollingTracker)
	} else {
		l.rollingTracker = nil
		l.status.SetRollingTracker(nil)
	}
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

	if changed {
		logger.Loop().Info("connection status changed", "up", l.status.Up)
		l.handleStateChange(ctx, upStatus)
	}

	l.nextCheckAt = time.Now().Add(l.delays.ForStatus(l.status.Up))
	l.pushStatus()
}

// Run starts the monitoring loop with optional statistics server config.
func (l *Loop) Run(ctx context.Context, statServerConfig *status.StatServerConfig) {
	checker := LoopChecker{tracker: l.rollingTracker}

	if l.statServer == nil {
		l.statServer = status.StartStatServer(l.status, statServerConfig)
	}

	for {
		checkStatus := check.CheckerRun(ctx, checker, l.checkList.All())
		if checkStatus {
			l.lastSuccess = time.Now()
		}

		l.ProcessCheck(ctx, checkStatus)

		sleepTime := l.delays.ForStatus(l.status.Up)
		logger.Loop().Debug("waiting for next loop iteration", "wait", sleepTime)

		select {
		case <-ctx.Done():
			logger.Loop().Debug("context canceled during sleep, exiting Run()")

			return
		case <-time.After(sleepTime):
		}
	}
}

// Stop gracefully shuts down the loop and its components.
func (l *Loop) Stop(ctx context.Context) {
	l.DownActionStop(ctx)

	if l.statServer != nil {
		l.statServer.Shutdown(ctx)
		l.statServer = nil
	}
}

func (l *Loop) handleStateChange(ctx context.Context, upStatus bool) {
	if l.downAction == nil {
		return
	}

	if upStatus {
		l.DownActionStop(ctx)
	} else {
		err := l.DownActionStart(ctx)
		if err != nil {
			logger.Loop().Error("could not start DownAction", "error", err)
		}
	}
}

func (l *Loop) pushStatus() {
	loopSt := status.LoopStatus{
		Interval: status.ReadableDuration(l.delays.ForStatus(l.status.Up)),
	}

	l.status.SetLoopStatus(loopSt)
	l.status.SetNextCheckAt(l.nextCheckAt)

	if !l.lastSuccess.IsZero() {
		l.status.SetLastSuccessAt(l.lastSuccess)
	}

	if l.downActionLoop != nil {
		l.status.SetDownActionStatus(l.downActionLoop.Status())
	} else {
		l.status.SetDownActionStatus(status.DownActionStatus{})
	}
}

// LoopChecker implements check.Checker for logging check lifecycle events and
// probe-level stats collection.
type LoopChecker struct {
	tracker *status.RollingProbeTracker
}

// CheckRun logs the start of a check.
func (LoopChecker) CheckRun(chk check.Check) {
	logger.Check().Debug("running",
		"probe",
		chk.Probe,
		"protocol",
		chk.Probe.Scheme(),
		"timeout",
		chk.Timeout,
	)
}

// ProbeSuccess logs successful probe results.
func (c LoopChecker) ProbeSuccess(report *check.Report) {
	logger.Check().Debug("success", report.LogAttrs())

	if c.tracker != nil {
		c.tracker.Record(false)
	}
}

// ProbeFailure logs failed probe results.
func (c LoopChecker) ProbeFailure(report *check.Report) {
	logger.Check().Warn("failed", report.LogAttrs())

	if c.tracker != nil {
		c.tracker.Record(true)
	}
}
