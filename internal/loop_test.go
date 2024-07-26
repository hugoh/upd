package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DownActionStartStop(t *testing.T) {
	da := getTestDA()
	loop := &Loop{
		DownAction: da,
	}
	assert.Nil(t, loop.downActionLoop)
	loop.DownActionStop()
	assert.Nil(t, loop.downActionLoop)
	err := loop.DownActionStart()
	assert.NoError(t, err)
	assert.NotNil(t, loop.downActionLoop)
	err = loop.DownActionStart()
	assert.Error(t, err)
	loop.DownActionStop()
	assert.Nil(t, loop.downActionLoop)
}