package internal

import "github.com/sirupsen/logrus"

var logger = logrus.New() //nolint:gochecknoglobals

func LogSetup(debugFlag bool) {
	if debugFlag {
		SetLogLevels(logrus.DebugLevel)
	}
}

// Sets both the local and global log levels
func SetLogLevels(level logrus.Level) {
	logger.SetLevel(level)
	logrus.SetLevel(level)
}
