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
		name    string
		text    string
		want    time.Duration
		wantErr string
	}{
		{name: "seconds", text: "10s", want: 10 * time.Second},
		{name: "minutes", text: "5m", want: 5 * time.Minute},
		{name: "hours", text: "1h30m", want: 90 * time.Minute},
		{name: "milliseconds", text: "500ms", want: 500 * time.Millisecond},
		{name: "zero", text: "0s", want: 0},
		{name: "invalid", text: "abc", wantErr: "invalid duration"},
		{name: "empty", text: "", wantErr: "invalid duration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration

			err := d.UnmarshalText([]byte(tt.text))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, Duration(tt.want), d)
			assert.Equal(t, tt.want, d.StdDuration())
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
