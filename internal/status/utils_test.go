package status

import (
	"encoding/json"
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

func TestReadablePercent_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		value    ReadablePercent
		expected string
	}{
		{"zero percent", 0.0, `"0.00 %"`},
		{"fifty percent", 0.5, `"50.00 %"`},
		{"hundred percent", 1.0, `"100.00 %"`},
		{"not computed", -1.0, `"Not computed"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.value)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestReadableDuration_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		value    ReadableDuration
		expected string
	}{
		{"zero", 0, `"0s"`},
		{"one second", ReadableDuration(time.Second), `"1s"`},
		{"one minute", ReadableDuration(time.Minute), `"1m"`},
		{"one hour", ReadableDuration(time.Hour), `"1h"`},
		{"complex", ReadableDuration(time.Hour + time.Minute + time.Second), `"1h1m1s"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.value)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}
