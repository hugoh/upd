// Package logger provides the global logger for the application.
package logger

import "github.com/sirupsen/logrus"

// L is the global logger instance for the application.
//
//nolint:gochecknoglobals // Standard pattern for application-wide logger
var L = logrus.New()

// LogSetup configures the logger based on the debug flag.
func LogSetup(debugFlag bool) {
	if debugFlag {
		L.SetLevel(logrus.DebugLevel)
	}
}
