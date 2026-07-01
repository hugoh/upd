package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogSetup_DebugFlagTrue(t *testing.T) {
	defer SetLevel(slog.LevelInfo)

	LogSetup(true)
	assert.True(t, L.Enabled(t.Context(), slog.LevelDebug))
}

func TestLogSetup_DebugFlagFalse(t *testing.T) {
	defer SetLevel(slog.LevelInfo)

	SetLevel(slog.LevelInfo)
	LogSetup(false)
	assert.False(t, L.Enabled(t.Context(), slog.LevelDebug))
	assert.True(t, L.Enabled(t.Context(), slog.LevelInfo))
}

func TestComponentAccessors(t *testing.T) {
	tests := []struct {
		name     string
		accessor func() *slog.Logger
	}{
		{"Check", Check},
		{"DownAction", DownAction},
		{"Loop", Loop},
		{"Stats", Stats},
		{"Config", Config},
		{"App", App},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := tt.accessor()
			require.NotNil(t, logger)
			assert.Same(
				t,
				logger,
				tt.accessor(),
				"accessor should return the same package-level logger instance every call",
			)
		})
	}
}

func TestSetLevel_AppliesToExistingLoggers(t *testing.T) {
	defer SetLevel(slog.LevelInfo)

	chk := Check()
	assert.False(t, chk.Enabled(t.Context(), slog.LevelDebug))

	SetLevel(slog.LevelDebug)
	assert.True(t, chk.Enabled(t.Context(), slog.LevelDebug),
		"level change should apply to loggers handed out earlier")
}
