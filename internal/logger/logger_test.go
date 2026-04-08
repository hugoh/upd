package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogSetup_DebugFlagTrue(t *testing.T) {
	originalLogger := L

	defer func() { L = originalLogger }()

	LogSetup(true)
	assert.NotNil(t, L)
}

func TestLogSetup_DebugFlagFalse(t *testing.T) {
	originalLogger := L

	defer func() { L = originalLogger }()

	L = slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	LogSetup(false)
	assert.NotNil(t, L)
}
