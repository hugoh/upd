package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestSetLevel_AppliesToExistingLoggers(t *testing.T) {
	defer SetLevel(slog.LevelInfo)

	chk := Check()
	assert.False(t, chk.Enabled(t.Context(), slog.LevelDebug))

	SetLevel(slog.LevelDebug)
	assert.True(t, chk.Enabled(t.Context(), slog.LevelDebug),
		"level change should apply to loggers handed out earlier")
}
