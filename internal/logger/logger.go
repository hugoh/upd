package logger

import "github.com/sirupsen/logrus"

// L is the global logger instance for the application.
//
//nolint:gochecknoglobals // Standard pattern for application-wide logger
var L = logrus.New()

func LogSetup(debugFlag bool) {
	if debugFlag {
		L.SetLevel(logrus.DebugLevel)
	}
}
