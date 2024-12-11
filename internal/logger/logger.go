package logger

import "github.com/sirupsen/logrus"

var Logger = logrus.New() //nolint:gochecknoglobals

func LogSetup(debugFlag bool) {
	if debugFlag {
		Logger.SetLevel(logrus.DebugLevel)
	}
}
