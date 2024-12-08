package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func emptyNewLoop() *Loop {
	return NewLoop(nil, nil, nil, false, NewStatus(TestVersion, 0))
}

func Test_ReportUpNess(t *testing.T) {
	var l *Loop
	var s bool
	l = emptyNewLoop()
	s = l.reportUpness(true)
	assert.True(t, s)
	s = l.reportUpness(true)
	assert.False(t, s)
	s = l.reportUpness(false)
	assert.True(t, s)
	l = emptyNewLoop()
	s = l.reportUpness(false)
	assert.True(t, s)
	s = l.reportUpness(true)
	assert.True(t, s)
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
