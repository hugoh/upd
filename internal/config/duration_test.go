package config

import (
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurationUnmarshalText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want time.Duration
	}{
		{"seconds", "10s", 10 * time.Second},
		{"minutes", "5m", 5 * time.Minute},
		{"hours", "1h30m", 90 * time.Minute},
		{"milliseconds", "500ms", 500 * time.Millisecond},
		{"zero", "0s", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration

			err := d.UnmarshalText([]byte(tt.text))
			require.NoError(t, err)
			assert.Equal(t, Duration(tt.want), d)
			assert.Equal(t, tt.want, d.StdDuration())
		})
	}
}

func TestDurationUnmarshalText_errors(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr string
	}{
		{"invalid", "abc", "invalid duration"},
		{"empty", "", "invalid duration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration

			err := d.UnmarshalText([]byte(tt.text))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestDurationUnmarshalText_struct(t *testing.T) {
	type testCfg struct {
		Timeout Duration `toml:"timeout"`
	}

	tomlData := `timeout = "30s"`

	var cfg testCfg

	err := toml.Unmarshal([]byte(tomlData), &cfg)
	require.NoError(t, err)
	assert.Equal(t, Duration(30*time.Second), cfg.Timeout)
}

func TestDurationStdDuration(t *testing.T) {
	d := Duration(5 * time.Minute)

	assert.Equal(t, 5*time.Minute, d.StdDuration())
	assert.IsType(t, time.Duration(0), d.StdDuration())
}
