package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewStatus(t *testing.T) {
	a := NewStatus(TestVersion)
	a.SetRetention(0)
	assert.NotNil(t, a)
	assert.Nil(t, a.StateChangeTracker)
	a = NewStatus(TestVersion)
	a.SetRetention(1 * time.Hour)
	assert.NotNil(t, a)
	assert.NotNil(t, a.StateChangeTracker)
}

func Test_Status(t *testing.T) {
	status := NewStatus(TestVersion)
	status.SetRetention(0)
	assert.False(t, status.Initialized)
	status.set(true)
	assert.True(t, status.Initialized)
	assert.True(t, status.Up)
	status.set(false)
	assert.True(t, status.Initialized)
	assert.False(t, status.Up)
	status = NewStatus(TestVersion)
	status.SetRetention(0)
	assert.False(t, status.Initialized)
	status.set(false)
	assert.True(t, status.Initialized)
	assert.False(t, status.Up)
	status.set(true)
	assert.True(t, status.Initialized)
	assert.True(t, status.Up)
}

func Test_Update(t *testing.T) {
	var s *Status
	var c bool
	s = NewStatus(TestVersion)
	s.SetRetention(0)
	c = s.Update(true)
	assert.True(t, c)
	c = s.Update(true)
	assert.False(t, c)
	c = s.Update(false)
	assert.True(t, c)
	s = NewStatus(TestVersion)
	s.SetRetention(0)
	c = s.Update(false)
	assert.True(t, c)
	c = s.Update(true)
	assert.True(t, c)
}
