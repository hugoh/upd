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
