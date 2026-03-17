// Package nulllogger provides a logger for testing that discards output.
package nulllogger

import (
	"io"

	"github.com/hugoh/upd/internal/logger"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

// NewNullLoggerHook creates a logger hook with discarded output for testing.
func NewNullLoggerHook() *test.Hook {
	logger.L = logrus.New()
	logger.L.Out = io.Discard

	return test.NewLocal(logger.L)
}
