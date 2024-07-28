package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ReportUpNess(t *testing.T) {
	var l *Loop
	var s bool
	l = &Loop{}
	s = l.reportUpness(true)
	assert.True(t, l.initialized)
	assert.True(t, l.isUp)
	assert.True(t, s)
	s = l.reportUpness(true)
	assert.True(t, l.initialized)
	assert.True(t, l.isUp)
	assert.False(t, s)
	s = l.reportUpness(false)
	assert.True(t, l.initialized)
	assert.False(t, l.isUp)
	assert.True(t, s)
	l = &Loop{}
	s = l.reportUpness(false)
	assert.True(t, l.initialized)
	assert.False(t, l.isUp)
	assert.True(t, s)
	s = l.reportUpness(true)
	assert.True(t, l.initialized)
	assert.True(t, l.isUp)
	assert.True(t, s)
}

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
