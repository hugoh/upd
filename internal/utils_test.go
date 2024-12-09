package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_FormatDuration(t *testing.T) {
	assert.Equal(t, "0s", formatDuration(0))
	assert.Equal(t, "0s", formatDuration(time.Millisecond))
	assert.Equal(t, "1s", formatDuration(time.Second))
	assert.Equal(t, "1s", formatDuration(time.Second+time.Millisecond))
	assert.Equal(t, "10s", formatDuration(10*time.Second))
	assert.Equal(t, "1m", formatDuration(time.Minute))
	assert.Equal(t, "10m", formatDuration(10*time.Minute))
	assert.Equal(t, "1m1s", formatDuration(time.Minute+time.Second))
	assert.Equal(t, "1h", formatDuration(time.Hour))
	assert.Equal(t, "1h0m1s", formatDuration(time.Hour+time.Second))
	assert.Equal(t, "1h1m", formatDuration(time.Hour+time.Minute))
	assert.Equal(t, "1h1m1s", formatDuration(time.Hour+time.Minute+time.Second))
}
