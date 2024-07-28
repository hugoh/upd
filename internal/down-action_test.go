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
	dal, _ := da.NewDownActionLoop()
	err := dal.Execute(da.Exec)
	assert.NoError(t, err)
}

//FIXME: Add test for iteration count as environment variable
//FIXME: Add test for StopExec

func Test_ExecuteFail(t *testing.T) {
	da := &DownAction{
		Exec: "false",
	}
	dal, _ := da.NewDownActionLoop()
	err := dal.Execute(da.Exec)
	assert.NoError(t, err, "Success in starting a command that fails")
}

func Test_ExecuteNonExistent(t *testing.T) {
	da := &DownAction{
		Exec: "/DOES-NOT-EXIST",
	}
	dal, _ := da.NewDownActionLoop()
	err := dal.Execute(da.Exec)
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
	dal := da.Start()
	assert.Equal(t, da, dal.da)
	assert.NotNil(t, dal.cancelFunc)
}

func Test_StartAndStop(t *testing.T) {
	every := 100 * time.Millisecond
	da := &DownAction{
		After: 1 * time.Millisecond,
		Every: every,
		Exec:  "true",
	}
	dal := da.Start()
	assert.NotNil(t, dal, "DownAction loop is running")
	time.Sleep(every) // Give it time to start
	assert.LessOrEqual(t, 0, dal.it.iteration, "DownAction loop is running")
	dal.Stop()
}

func testBackoff(t *testing.T, hasLimit bool) {
	da := getTestDA()
	const backoffLimit = 2 * time.Second
	if hasLimit {
		da.BackoffLimit = backoffLimit
	}
	assert.Equal(t, 1.5, BackoffFactor, "Ensuring we have the right values")
	dal, _ := da.NewDownActionLoop()
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
