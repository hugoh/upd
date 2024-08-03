package internal

import "github.com/sirupsen/logrus"

var logger = logrus.New() //nolint:gochecknoglobals

func LogSetup(debugFlag bool) {
	if debugFlag {
		logger.SetLevel(logrus.DebugLevel)
	}
}
