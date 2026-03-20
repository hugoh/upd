package nulllogger

import (
	"io"
	"testing"

	"github.com/hugoh/upd/internal/logger"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewNullLoggerHook(t *testing.T) {
	hook := NewNullLoggerHook()
	assert.NotNil(t, hook)

	assert.Equal(t, io.Discard, logger.L.Out)
}

func TestNewNullLoggerHook_CapturesEntries(t *testing.T) {
	hook := NewNullLoggerHook()
	logger.L.Info("test message")

	entries := hook.AllEntries()
	assert.Len(t, entries, 1)
	assert.Equal(t, logrus.InfoLevel, entries[0].Level)
	assert.Equal(t, "test message", entries[0].Message)
}
