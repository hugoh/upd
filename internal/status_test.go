package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewStatus(t *testing.T) {
	a := NewStatus(TestVersion, 0)
	assert.NotNil(t, a)
	assert.Nil(t, a.StateChangeTracker)
	a = NewStatus(TestVersion, 1*time.Hour)
	assert.NotNil(t, a)
	assert.NotNil(t, a.StateChangeTracker)
}

func Test_Status(t *testing.T) {
	status := NewStatus(TestVersion, 0)
	assert.False(t, status.Initialized)
	status.Set(true)
	assert.True(t, status.Initialized)
	assert.True(t, status.Up)
	status.Set(false)
	assert.True(t, status.Initialized)
	assert.False(t, status.Up)
	status = NewStatus(TestVersion, 0)
	assert.False(t, status.Initialized)
	status.Set(false)
	assert.True(t, status.Initialized)
	assert.False(t, status.Up)
	status.Set(true)
	assert.True(t, status.Initialized)
	assert.True(t, status.Up)
}
