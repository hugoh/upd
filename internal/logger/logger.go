package logger

import "github.com/sirupsen/logrus"

var L = logrus.New() //nolint:gochecknoglobals

func LogSetup(debugFlag bool) {
	if debugFlag {
		L.SetLevel(logrus.DebugLevel)
	}
}
