package logic

import (
	"context"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/nulllogger"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

// FIXME: waiting for commands with time.Sleep() to run is error-prone

func ensureExec(t *testing.T, hook *test.Hook, expectedValue string, foundCount int) {
	count := 0
	for _, e := range hook.AllEntries() {
		if val, ok := e.Data["exec"]; ok {
			count++
			assert.Equal(t, expectedValue, val, "exec doesn't match")
			if count == foundCount {
				return
			}
		}
	}
	assert.Equal(t, foundCount, count, "Could not find enough entries")
}

func Test_ExecuteSucceed(t *testing.T) {
	da := &DownAction{
		Exec: "true",
	}
	hook := nulllogger.NewNullLoggerHook()
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(da.Exec)
	assert.NoError(t, err)
	ensureExec(t, hook, "/usr/bin/true", 1)
}

func Test_ExecuteFail(t *testing.T) {
	da := &DownAction{
		Exec: "false",
	}
	hook := nulllogger.NewNullLoggerHook()
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(da.Exec)
	assert.NoError(t, err, "Success in starting a command that fails")
	ensureExec(t, hook, "/usr/bin/false", 1)
	time.Sleep(50 * time.Millisecond) // Give it time to fail
	ensureExec(t, hook, "/usr/bin/false", 1)
}

func Test_ExecuteNonExistent(t *testing.T) {
	da := &DownAction{
		Exec: "/DOES-NOT-EXIST",
	}
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute(da.Exec)
	assert.Error(t, err)
}

func Test_ExecuteEmptyCommand(t *testing.T) {
	da := &DownAction{} // Exec is empty by default
	dal, _ := da.NewDownActionLoop(context.Background())
	err := dal.Execute("") // Explicitly pass empty string
	assert.Error(t, err)
	assert.Equal(t, ErrNoCommand, err)
}

// ensureCommandLogSequence checks if the logged "exec" commands match the expected sequence.
func ensureCommandLogSequence(t *testing.T, hook *test.Hook, expectedCmdSequence []string) {
	loggedCmds := []string{}
	// Give a little time for all logs to be processed, especially async warn logs from cmd.Wait()
	time.Sleep(50 * time.Millisecond) 
	for _, entry := range hook.AllEntries() {
		if cmd, ok := entry.Data["exec"]; ok {
			if cmdStr, okStr := cmd.(string); okStr {
				loggedCmds = append(loggedCmds, cmdStr)
			}
		}
	}
	assert.Equal(t, expectedCmdSequence, loggedCmds, "Logged command sequence does not match expected. Got: %v", loggedCmds)
}

func TestDownAction_StartAndStop_WithStopExec(t *testing.T) {
	const execCmdPath = "/usr/bin/true"
	const stopExecCmdPath = "/usr/bin/false"

	da := &DownAction{
		After:    1 * time.Millisecond, // Run Exec quickly
		Every:    100 * time.Second,    // Effectively run Exec only once before stop
		Exec:     "true",               // Command that resolves to /usr/bin/true (exits 0)
		StopExec: "false",              // Command that resolves to /usr/bin/false (exits 1)
	}

	hook := nulllogger.NewNullLoggerHook()
	dal := da.Start(context.Background())
	assert.NotNil(t, dal, "DownAction loop should be running")

	// Allow time for Exec command to run and be logged.
	// dal.da.After is 1ms. Add some buffer.
	time.Sleep(50 * time.Millisecond)

	dal.Stop() // This will trigger StopExec

	// After Stop(), StopExec is executed. If it fails (like "false"), it's logged twice.
	// Exec ("true") is logged once (INFO).
	// StopExec ("false") is logged once (INFO) during Execute by Stop().
	// StopExec ("false") is logged again (WARN) by the cmd.Wait() goroutine because "false" exits with error.
	expectedSequence := []string{execCmdPath, stopExecCmdPath, stopExecCmdPath}

	// The ensureCommandLogSequence includes a small delay to catch all logs.
	ensureCommandLogSequence(t, hook, expectedSequence)
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
}

func Test_StartAndStop(t *testing.T) {
	waitTime := 50 * time.Millisecond
	every := 100 * time.Millisecond
	da := &DownAction{
		After:    1 * time.Millisecond,
		Every:    every,
		Exec:     "true",
		StopExec: "false",
	}
	hook := nulllogger.NewNullLoggerHook()
	dal := da.Start(context.Background())
	assert.NotNil(t, dal, "DownAction loop is running")
	time.Sleep(waitTime) // Give it time to startExec
	ensureExec(t, hook, "/usr/bin/true", 1)
	hook.Reset()
	dal.Stop()
	time.Sleep(every) // Give it time to stop
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
	assert.Equal(t, DaIteration{iteration: -1, sleepTime: 0}, *dal.it)
	dal.iterate()
	assert.Equal(t, DaIteration{iteration: 0, sleepTime: da.After}, *dal.it)
	dal.iterate()
	assert.Equal(t, DaIteration{iteration: 1, sleepTime: da.Every}, *dal.it)
	dal.iterate()
	current := time.Duration(1.5 * float64(time.Second))
	assert.Equal(t, DaIteration{iteration: 2, sleepTime: current}, *dal.it)
	dal.iterate()
	if hasLimit {
		current = da.BackoffLimit
	} else {
		current = time.Duration(2.25 * float64(time.Second))
	}
	assert.Equal(t, DaIteration{iteration: 3, sleepTime: current, limitReached: hasLimit}, *dal.it)
}

func Test_BackoffNoLimit(t *testing.T) {
	testBackoff(t, false)
}

func Test_BackoffLimit(t *testing.T) {
	testBackoff(t, true)
}
