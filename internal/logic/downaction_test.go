package logic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTrue  = "true"
	testFalse = "false"
)

func Test_ExecuteSucceed(t *testing.T) {
	da := &DownAction{
		Exec: testTrue,
	}
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(context.Background(), da.Exec)
	require.NoError(t, err)
}

func Test_ExecuteFail(t *testing.T) {
	da := &DownAction{
		Exec: testFalse,
	}
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(context.Background(), da.Exec)
	require.NoError(t, err, "Success in starting a command that fails")
}

func Test_ExecuteNonExistent(t *testing.T) {
	da := &DownAction{
		Exec: "/DOES-NOT-EXIST",
	}
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(context.Background(), da.Exec)
	require.Error(t, err)
}

func Test_ExecuteStderrCapture(t *testing.T) {
	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(context.Background())

	err := dal.Execute(context.Background(), "sh -c 'echo stderr-output >&2; exit 1'")
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
}

func Test_ExecuteStderrCapture_Trimmed(t *testing.T) {
	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(context.Background())

	err := dal.Execute(context.Background(),
		"sh -c 'printf \"\n\n  spaced-stderr  \n\n\" >&2; exit 1'")
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
}

func Test_killCurrentCmd_nilCmd(t *testing.T) {
	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(context.Background())

	require.NotPanics(t, func() { dal.killCurrentCmd() })

	dal.cmdMu.Lock()
	assert.Nil(t, dal.currentCmd)
	dal.cmdMu.Unlock()
}

func Test_killCurrentCmd_killsRunning(t *testing.T) {
	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(context.Background())

	err := dal.Execute(context.Background(), "sleep 10")
	require.NoError(t, err)

	dal.cmdMu.Lock()
	require.NotNil(t, dal.currentCmd)
	dal.cmdMu.Unlock()

	dal.killCurrentCmd()

	dal.cmdMu.Lock()
	assert.Nil(t, dal.currentCmd)
	dal.cmdMu.Unlock()
}

func Test_ExecuteReplacesStaleCmd(t *testing.T) {
	da := &DownAction{}
	dal, _ := da.NewDownActionLoop(context.Background())

	err := dal.Execute(context.Background(), "sleep 10")
	require.NoError(t, err)

	dal.cmdMu.Lock()
	require.NotNil(t, dal.currentCmd)
	firstPid := dal.currentCmd.Process.Pid
	dal.cmdMu.Unlock()

	dal.killCurrentCmd()

	dal.cmdMu.Lock()
	assert.Nil(t, dal.currentCmd)
	dal.cmdMu.Unlock()

	err = dal.Execute(context.Background(), "true")
	require.NoError(t, err)

	dal.cmdMu.Lock()
	require.NotNil(t, dal.currentCmd)
	assert.NotEqual(t, firstPid, dal.currentCmd.Process.Pid, "Should track new process")
	dal.cmdMu.Unlock()

	time.Sleep(200 * time.Millisecond)
}

func Test_StopUsesLoopCtx(t *testing.T) {
	da := &DownAction{}
	dal, ctx := da.NewDownActionLoop(context.Background())

	assert.NotNil(t, dal.loopCtx, "loopCtx should be stored")
	assert.Equal(t, ctx, dal.loopCtx, "loopCtx matches returned child context")

	dal.Stop(context.Background())

	select {
	case <-dal.loopCtx.Done():
	case <-time.After(time.Second):
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
	dal := da.Start(context.Background())
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
	dal := da.Start(context.Background())
	assert.NotNil(t, dal, "DownAction loop is running")
	time.Sleep(50 * time.Millisecond)
	dal.Stop(context.Background())
}

func testBackoff(t *testing.T, hasLimit bool) {
	t.Helper()

	da := getTestDA()

	const backoffLimit = 2 * time.Second
	if hasLimit {
		da.BackoffLimit = backoffLimit
	}

	assert.InEpsilon(t, 1.5, BackoffFactor, 0.01, "Ensuring we have the right values")

	dal, _ := da.NewDownActionLoop(context.Background())
	assert.Equal(t, DownActionIteration{iteration: -1, sleepTime: 0}, *dal.it)
	dal.iterate()
	assert.Equal(t, DownActionIteration{iteration: 0, sleepTime: da.After}, *dal.it)
	dal.iterate()
	assert.Equal(t, DownActionIteration{iteration: 1, sleepTime: da.Every}, *dal.it)
	dal.iterate()

	current := time.Duration(1.5 * float64(time.Second))
	assert.Equal(t, DownActionIteration{iteration: 2, sleepTime: current}, *dal.it)
	dal.iterate()

	if hasLimit {
		current = da.BackoffLimit
	} else {
		current = time.Duration(2.25 * float64(time.Second))
	}

	assert.Equal(
		t,
		DownActionIteration{iteration: 3, sleepTime: current, limitReached: hasLimit},
		*dal.it,
	)
}

func Test_BackoffNoLimit(t *testing.T) {
	testBackoff(t, false)
}

func Test_BackoffLimit(t *testing.T) {
	testBackoff(t, true)
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
