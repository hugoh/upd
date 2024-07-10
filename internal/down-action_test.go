package internal

import (
	"testing"

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
