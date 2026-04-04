package logic

import (
	"context"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/nulllogger"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func ensureExec(t *testing.T, hook *test.Hook, expectedValue string, foundCount int) {
	assert.Eventually(t, func() bool {
		count := 0
		for _, e := range hook.AllEntries() {
			if val, ok := e.Data["exec"]; ok {
				count++
				if val != expectedValue {
					return false
				}
				if count == foundCount {
					return true
				}
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond, "Expected to find %d entries with exec=%s", foundCount, expectedValue)
}

func Test_ExecuteSucceed(t *testing.T) {
	da := &DownAction{
		Exec: "true",
	}
	hook := nulllogger.NewNullLoggerHook()
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(context.Background(), da.Exec)
	assert.NoError(t, err)
	ensureExec(t, hook, "/usr/bin/true", 1)
}

func Test_ExecuteFail(t *testing.T) {
	da := &DownAction{
		Exec: "false",
	}
	hook := nulllogger.NewNullLoggerHook()
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(context.Background(), da.Exec)
	assert.NoError(t, err, "Success in starting a command that fails")
	ensureExec(t, hook, "/usr/bin/false", 1)
}

func Test_ExecuteNonExistent(t *testing.T) {
	da := &DownAction{
		Exec: "/DOES-NOT-EXIST",
	}
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(context.Background(), da.Exec)
	assert.Error(t, err)
}

func getTestDA() *DownAction {
	const after = 42 * time.Second
	const every = 1 * time.Second
	return &DownAction{
		After: after,
		Every: every,
		Exec:  "true",
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
	time.Sleep(50 * time.Millisecond)
	every := 100 * time.Millisecond
	da := &DownAction{
		After:    10 * time.Millisecond,
		Every:    every,
		Exec:     "true",
		StopExec: "false",
	}
	hook := nulllogger.NewNullLoggerHook()
	dal := da.Start(context.Background())
	assert.NotNil(t, dal, "DownAction loop is running")
	time.Sleep(50 * time.Millisecond)
	ensureExec(t, hook, "/usr/bin/true", 1)
	hook.Reset()
	dal.Stop(context.Background())
	ensureExec(t, hook, "/usr/bin/false", 2)
}

func testBackoff(t *testing.T, hasLimit bool) {
	da := getTestDA()
	const backoffLimit = 2 * time.Second
	if hasLimit {
		da.BackoffLimit = backoffLimit
	}
	assert.Equal(t, 1.5, BackoffFactor, "Ensuring we have the right values")
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
	assert.Equal(t, DownActionIteration{iteration: 3, sleepTime: current, limitReached: hasLimit}, *dal.it)
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
			command:     []string{"true"},
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
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr, err, "Error should match expected")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
