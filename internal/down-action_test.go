package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_ExecuteSucceed(t *testing.T) {
	da := &DownAction{
		Exec: "true",
	}
	err := da.Execute()
	assert.NoError(t, err)
}

func Test_ExecuteFail(t *testing.T) {
	da := &DownAction{
		Exec: "false",
	}
	err := da.Execute()
	assert.NoError(t, err, "Success in starting a command that fails")
}

func Test_ExecuteNonExistent(t *testing.T) {
	da := &DownAction{
		Exec: "/DOES-NOT-EXIST",
	}
	err := da.Execute()
	assert.Error(t, err)
}

func Test_StartAndStop(t *testing.T) {
	every := 100 * time.Millisecond
	da := &DownAction{
		After: 1 * time.Millisecond,
		Every: every,
		Exec:  "true",
	}
	da.Start()
	time.Sleep(every) // Give it time to start
	assert.Equal(t, true, da.isRunning(), "DownAction loop is running")
	da.Stop()
	time.Sleep(every) // Give it time to stop
	assert.Equal(t, false, da.isRunning(), "DownAction loop is not running anymore")
}

func test_backoff(t *testing.T, hasLimit bool) {
	const after = 42 * time.Second
	const every = 1 * time.Second
	const backoffLimit = 2 * time.Second
	var limit time.Duration
	if hasLimit {
		limit = backoffLimit
	} else {
		limit = 0
	}
	assert.Equal(t, 1.5, BackoffFactor, "Ensuring we have the right values")
	da := &DownAction{
		After:        after,
		Every:        every,
		Exec:         "true",
		BackoffLimit: limit,
	}
	it := &DaIteration{}
	assert.Equal(t, DaIteration{Iteration: 0, SleepTime: 0}, *it)
	it.iterate(da)
	assert.Equal(t, DaIteration{Iteration: 1, SleepTime: after}, *it)
	it.iterate(da)
	assert.Equal(t, DaIteration{Iteration: 2, SleepTime: every}, *it)
	it.iterate(da)
	current := time.Duration(1.5 * float64(time.Second))
	assert.Equal(t, DaIteration{Iteration: 3, SleepTime: current}, *it)
	it.iterate(da)
	if hasLimit {
		current = limit
	} else {
		current = time.Duration(2.25 * float64(time.Second))
	}
	assert.Equal(t, DaIteration{Iteration: 4, SleepTime: current, LimitReached: hasLimit}, *it)
}

func Test_BackoffNoLimit(t *testing.T) {
	test_backoff(t, false)
}

func Test_BackoffLimit(t *testing.T) {
	test_backoff(t, true)
}
