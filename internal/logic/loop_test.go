package logic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

const TestVersion = "test"

func emptyNewLoop() *Loop {
	l := NewLoop(TestVersion)
	l.Configure(nil, nil, nil, false, 0, nil)
	return l
}

func Test_DownActionStartStop(t *testing.T) {
	ctx := context.Background()
	da := getTestDA()
	loop := emptyNewLoop()
	loop.downAction = da
	assert.Nil(t, loop.downActionLoop)
	loop.DownActionStop()
	assert.Nil(t, loop.downActionLoop)
	err := loop.DownActionStart(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, loop.downActionLoop)
	err = loop.DownActionStart(ctx)
	assert.Error(t, err)
	loop.DownActionStop()
	assert.Nil(t, loop.downActionLoop)
}

func Test_ProcessCheck_StatusNotChanged(t *testing.T) {
	loop := emptyNewLoop()
	// Status.Up is false by default, so passing false should not change it
	ctx := context.Background()
	loop.ProcessCheck(ctx, false)
	// No change, so DownAction should not be started/stopped
	assert.Nil(t, loop.downActionLoop)
}

func Test_ProcessCheck_StatusChanged_NoDownAction(t *testing.T) {
	loop := emptyNewLoop()
	// Status.Up is false by default, so passing true should change it
	ctx := context.Background()
	loop.downAction = nil // explicitly no DownAction
	loop.ProcessCheck(ctx, true)
	// DownAction is nil, so nothing should be started/stopped
	assert.Nil(t, loop.downActionLoop)
}

func Test_ProcessCheck_StatusChanged_UpStatus_StopsDownAction(t *testing.T) {
	loop := emptyNewLoop()
	ctx := context.Background()
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
	ctx := context.Background()
	da := getTestDA()
	loop.downAction = da
	// Status.Up is false by default, so first call with true to set Up=true
	loop.ProcessCheck(ctx, true)
	assert.Nil(t, loop.downActionLoop)
	// Now call with false, should trigger DownActionStart
	loop.ProcessCheck(ctx, false)
	assert.NotNil(t, loop.downActionLoop)
}

func Test_ProcessCheck_StatusChanged_DownStatus_StartsDownAction_Error(t *testing.T) {
	loop := emptyNewLoop()
	ctx := context.Background()
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
