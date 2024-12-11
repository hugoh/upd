package nulllogger

import (
	"io"

	"github.com/hugoh/upd/internal/logger"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

func NewNullLoggerHook() *test.Hook {
	logger.Logger = logrus.New()
	logger.Logger.Out = io.Discard
	return test.NewLocal(logger.Logger)
}
