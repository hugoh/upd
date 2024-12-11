package logic

import (
	"testing"

	"github.com/hugoh/upd/internal/status"
	"github.com/stretchr/testify/assert"
)

const TestVersion = "test"

func emptyNewLoop() *Loop {
	return NewLoop(nil, nil, nil, false, status.NewStatus(TestVersion, 0))
}

func Test_DownActionStartStop(t *testing.T) {
	da := getTestDA()
	loop := emptyNewLoop()
	loop.DownAction = da
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
