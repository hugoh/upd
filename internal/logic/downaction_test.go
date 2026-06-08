package logic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTrue  = "true"
	testFalse = "false"
)

func newRunningDAL(t *testing.T) *DownActionLoop {
	t.Helper()

	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(t.Context())

	err := dal.Execute(t.Context(), "sleep 10")
	require.NoError(t, err)

	return dal
}

func Test_ExecuteSucceed(t *testing.T) {
	da := &DownAction{
		Exec: testTrue,
	}
	dal, _ := da.NewDownActionLoop(t.Context())
	err := dal.Execute(t.Context(), da.Exec)
	require.NoError(t, err)
}

func Test_ExecuteFail(t *testing.T) {
	da := &DownAction{
		Exec: testFalse,
	}
	dal, _ := da.NewDownActionLoop(t.Context())
	err := dal.Execute(t.Context(), da.Exec)
	require.NoError(t, err, "Success in starting a command that fails")
}

func Test_ExecuteNonExistent(t *testing.T) {
	da := &DownAction{
		Exec: "/DOES-NOT-EXIST",
	}
	dal, _ := da.NewDownActionLoop(t.Context())
	err := dal.Execute(t.Context(), da.Exec)
	require.Error(t, err)
}

func Test_ExecuteStderrCapture(t *testing.T) {
	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(t.Context())

	err := dal.Execute(t.Context(), "sh -c 'echo stderr-output >&2; exit 1'")
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
}

func Test_ExecuteStderrCapture_Trimmed(t *testing.T) {
	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(t.Context())

	err := dal.Execute(t.Context(),
		"sh -c 'printf \"\n\n  spaced-stderr  \n\n\" >&2; exit 1'")
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
}

func Test_killCurrentCmd_nilCmd(t *testing.T) {
	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(t.Context())

	require.NotPanics(t, func() { dal.killCurrentCmd() })

	dal.cmdMu.Lock()
	assert.Nil(t, dal.currentCmd)
	dal.cmdMu.Unlock()
}

func Test_killCurrentCmd_killsRunning(t *testing.T) {
	dal := newRunningDAL(t)

	dal.cmdMu.Lock()
	require.NotNil(t, dal.currentCmd)
	dal.cmdMu.Unlock()

	dal.killCurrentCmd()

	dal.cmdMu.Lock()
	assert.Nil(t, dal.currentCmd)
	dal.cmdMu.Unlock()
}

func Test_ExecuteReplacesStaleCmd(t *testing.T) {
	dal := newRunningDAL(t)

	dal.cmdMu.Lock()
	firstPid := dal.currentCmd.Process.Pid
	dal.cmdMu.Unlock()

	dal.killCurrentCmd()

	dal.cmdMu.Lock()
	assert.Nil(t, dal.currentCmd)
	dal.cmdMu.Unlock()

	err := dal.Execute(t.Context(), "true")
	require.NoError(t, err)

	dal.cmdMu.Lock()
	require.NotNil(t, dal.currentCmd)
	assert.NotEqual(t, firstPid, dal.currentCmd.Process.Pid, "Should track new process")
	dal.cmdMu.Unlock()

	time.Sleep(200 * time.Millisecond)
}

func Test_StopUsesLoopCtx(t *testing.T) {
	da := &DownAction{}
	dal, ctx := da.NewDownActionLoop(t.Context())

	assert.NotNil(t, dal.loopCtx, "loopCtx should be stored")
	assert.Equal(t, ctx, dal.loopCtx, "loopCtx matches returned child context")

	dal.Stop(t.Context())

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	select {
	case <-dal.loopCtx.Done():
	case <-timer.C:
		t.Fatal("loopCtx should be cancelled after Stop")
	}
}

func getTestDA() *DownAction {
	const (
		after = 42 * time.Second
		every = 1 * time.Second
	)

	return &DownAction{
		After: after,
		Every: every,
		Exec:  testTrue,
	}
}

func Test_Start(t *testing.T) {
	da := getTestDA()
	dal := da.Start(t.Context())
	assert.Equal(t, da, dal.da)
	assert.NotNil(t, dal.cancelFunc)
	dal.cancelFunc()
}

func Test_StartAndStop(t *testing.T) {
	every := 100 * time.Millisecond
	da := &DownAction{
		After:    10 * time.Millisecond,
		Every:    every,
		Exec:     testTrue,
		StopExec: testTrue,
	}
	dal := da.Start(t.Context())
	assert.NotNil(t, dal, "DownAction loop is running")
	time.Sleep(50 * time.Millisecond)
	dal.Stop(t.Context())
}

func testBackoff(t *testing.T, hasLimit bool) {
	t.Helper()

	da := getTestDA()

	const backoffLimit = 2 * time.Second
	if hasLimit {
		da.BackoffLimit = backoffLimit
	}

	assert.InEpsilon(t, 1.5, BackoffFactor, 0.01, "Ensuring we have the right values")

	dal, _ := da.NewDownActionLoop(t.Context())
	assert.Equal(t, uint64(0), dal.iteration.Load())
	assert.Equal(t, da.After, dal.sleepTime)
	assert.False(t, dal.limitReached)

	sleepTime := dal.nextSleep()
	assert.Equal(t, uint64(1), dal.iteration.Load())
	assert.Equal(t, da.Every, sleepTime)

	sleepTime = dal.nextSleep()
	assert.Equal(t, uint64(2), dal.iteration.Load())
	assert.Equal(t, time.Duration(1.5*float64(time.Second)), sleepTime)

	sleepTime = dal.nextSleep()
	assert.Equal(t, uint64(3), dal.iteration.Load())

	if hasLimit {
		assert.Equal(t, da.BackoffLimit, sleepTime)
		assert.True(t, dal.limitReached)
	} else {
		assert.Equal(t, time.Duration(2.25*float64(time.Second)), sleepTime)
		assert.False(t, dal.limitReached)
	}
}

func Test_BackoffNoLimit(t *testing.T) {
	testBackoff(t, false)
}

func Test_BackoffLimit(t *testing.T) {
	testBackoff(t, true)
}

func Test_Status_Initial(t *testing.T) {
	da := &DownAction{Exec: testTrue}
	dal, _ := da.NewDownActionLoop(t.Context())

	st := dal.Status()
	assert.Equal(t, uint64(0), st.Iteration)
	assert.Zero(t, st.SleepTime)
	assert.False(t, st.BackoffCapped)
}

func Test_Status_AfterIteration(t *testing.T) {
	da := getTestDA()
	dal, _ := da.NewDownActionLoop(t.Context())

	dal.nextSleep()
	st := dal.Status()
	assert.Equal(t, uint64(1), st.Iteration)
	assert.Equal(t, da.Every, time.Duration(st.SleepTime))
	assert.False(t, st.BackoffCapped)

	dal.nextSleep()
	st = dal.Status()
	assert.Equal(t, uint64(2), st.Iteration)
	assert.InEpsilon(t, 1.5*float64(time.Second), float64(time.Duration(st.SleepTime)), 0.01)
	assert.False(t, st.BackoffCapped)
}

func Test_Status_BackoffCapped(t *testing.T) {
	da := getTestDA()
	da.BackoffLimit = 2 * time.Second
	dal, _ := da.NewDownActionLoop(t.Context())

	// After enough iterations, backoff hits the limit
	for range 5 {
		dal.nextSleep()
	}

	st := dal.Status()
	assert.True(t, st.BackoffCapped)
	assert.Equal(t, da.BackoffLimit, time.Duration(st.SleepTime))
}

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     []string
		expectedErr error
	}{
		{
			name:        "Valid command",
			command:     []string{"ls", "-la"},
			expectedErr: nil,
		},
		{
			name:        "Valid single command",
			command:     []string{testTrue},
			expectedErr: nil,
		},
		{
			name:        "Empty command slice",
			command:     []string{},
			expectedErr: ErrNoCommand,
		},
		{
			name:        "Nil command slice",
			command:     nil,
			expectedErr: ErrNoCommand,
		},
		{
			name:        "Empty command name",
			command:     []string{"", "arg"},
			expectedErr: ErrEmptyCommand,
		},
		{
			name:        "Command with just empty string",
			command:     []string{""},
			expectedErr: ErrEmptyCommand,
		},
		{
			name:        "Command with empty first element and args",
			command:     []string{"", "arg1", "arg2"},
			expectedErr: ErrEmptyCommand,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(tt.command)
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr, err, "Error should match expected")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
