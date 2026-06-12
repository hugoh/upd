package logic

import (
	"testing"
	"time"

	"github.com/hugoh/upd/internal/check"
	"github.com/hugoh/upd/internal/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func emptyNewLoop() *Loop {
	l := NewLoop()
	l.Configure(nil, Delays{}, nil, status.BucketConfig{})

	return l
}

func TestConfigure_TrackerCreatedOnlyWithReports(t *testing.T) {
	t.Run("no periods", func(t *testing.T) {
		loop := NewLoop()
		loop.Configure(nil, Delays{}, nil, status.BucketConfig{})
		assert.Nil(t, loop.rollingTracker, "no tracker without reports")
	})

	t.Run("with periods", func(t *testing.T) {
		loop := NewLoop()
		loop.Configure(nil, Delays{}, nil, status.BucketConfig{}, time.Minute, 5*time.Minute)
		assert.NotNil(t, loop.rollingTracker, "tracker created when reports given")
	})
}

func Test_DownActionStartStop(t *testing.T) {
	ctx := t.Context()
	da := getTestDA()
	loop := emptyNewLoop()
	loop.downAction = da
	assert.Nil(t, loop.downActionLoop)
	loop.DownActionStop(ctx)
	assert.Nil(t, loop.downActionLoop)
	err := loop.DownActionStart(ctx)
	require.NoError(t, err)
	assert.NotNil(t, loop.downActionLoop)
	err = loop.DownActionStart(ctx)
	require.Error(t, err)
	loop.DownActionStop(ctx)
	assert.Nil(t, loop.downActionLoop)
}

func Test_ProcessCheck_StatusNotChanged(t *testing.T) {
	loop := emptyNewLoop()
	// Status.Up is false by default, so passing false should not change it
	ctx := t.Context()
	loop.ProcessCheck(ctx, false)
	// No change, so DownAction should not be started/stopped
	assert.Nil(t, loop.downActionLoop)
}

func Test_ProcessCheck_StatusChanged_NoDownAction(t *testing.T) {
	loop := emptyNewLoop()
	// Status.Up is false by default, so passing true should change it
	ctx := t.Context()
	loop.downAction = nil // explicitly no DownAction
	loop.ProcessCheck(ctx, true)
	// DownAction is nil, so nothing should be started/stopped
	assert.Nil(t, loop.downActionLoop)
}

func Test_ProcessCheck_StatusChanged_UpStatus_StopsDownAction(t *testing.T) {
	loop := emptyNewLoop()
	ctx := t.Context()
	da := getTestDA()
	loop.downAction = da
	// Simulate DownAction already running
	_ = loop.DownActionStart(ctx)
	assert.NotNil(t, loop.downActionLoop)
	// Now, upStatus=true should stop DownAction
	loop.ProcessCheck(ctx, true)
	assert.Nil(t, loop.downActionLoop)
}

func Test_ProcessCheck_StatusChanged_DownStatus_StartsDownAction(t *testing.T) {
	loop := emptyNewLoop()
	ctx := t.Context()
	da := getTestDA()
	loop.downAction = da
	// Status.Up is false by default, so first call with true to set Up=true
	loop.ProcessCheck(ctx, true)
	assert.Nil(t, loop.downActionLoop)
	// Now call with false, should trigger DownActionStart
	loop.ProcessCheck(ctx, false)
	assert.NotNil(t, loop.downActionLoop)
}

func Test_ProcessCheck_PopulatesLoopStatus(t *testing.T) {
	loop := NewLoop()
	loop.Configure(nil, Delays{Up: time.Minute, Down: 30 * time.Second}, nil, status.BucketConfig{})

	ctx := t.Context()

	loop.lastSuccess = time.Now()
	// Status.Up is false by default, so true changes it
	loop.ProcessCheck(ctx, true)

	report := loop.status.GenStatReport(nil)
	require.NotNil(t, report.Loop)
	assert.Equal(t, time.Minute, time.Duration(report.Loop.Interval))
	nextCheck := time.Duration(report.Loop.NextCheck)
	assert.Greater(t, nextCheck, 50*time.Second)
	assert.LessOrEqual(t, nextCheck, time.Minute)
	assert.NotZero(t, time.Duration(report.Loop.LastSuccess))
}

func Test_ProcessCheck_PopulatesDownActionStatus(t *testing.T) {
	loop := NewLoop()
	da := getTestDA()
	loop.Configure(nil, Delays{Up: time.Minute, Down: 30 * time.Second}, da, status.BucketConfig{})

	ctx := t.Context()

	// Transition to up first (initialized state)
	loop.ProcessCheck(ctx, true)
	assert.Nil(t, loop.status.GenStatReport(nil).DownAction)

	// Transition to down — starts down action
	loop.ProcessCheck(ctx, false)
	report := loop.status.GenStatReport(nil)
	require.NotNil(t, report.DownAction)
	assert.Equal(t, uint32(0), report.DownAction.Iteration)
	assert.False(t, report.DownAction.BackoffCapped)
}

func Test_ProcessCheck_StatusChanged_DownStatus_StartsDownAction_Error(t *testing.T) {
	loop := emptyNewLoop()
	ctx := t.Context()
	// Use a DownAction that will simulate already running
	da := getTestDA()
	loop.downAction = da
	// Simulate DownAction already running
	_ = loop.DownActionStart(ctx)
	// Now, status change to down should try to start again, but error is handled internally
	loop.ProcessCheck(ctx, false)
	// DownAction should still be running
	assert.NotNil(t, loop.downActionLoop)
}

func TestChecker_CheckRun(t *testing.T) {
	checker := LoopChecker{}
	probe := check.Probe(check.NewHTTPProbe("http://example.com"))
	check := check.Check{Probe: probe, Timeout: time.Second}

	assert.NotPanics(t, func() {
		checker.CheckRun(check)
	})
}

func TestChecker_ProbeSuccess(t *testing.T) {
	checker := LoopChecker{}
	report := &check.Report{}

	assert.NotPanics(t, func() {
		checker.ProbeSuccess(report)
	})
}

func TestChecker_ProbeFailure(t *testing.T) {
	checker := LoopChecker{}
	report := &check.Report{}

	assert.NotPanics(t, func() {
		checker.ProbeFailure(report)
	})
}
