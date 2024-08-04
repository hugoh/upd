package internal

import (
	"log/syslog"

	"github.com/sirupsen/logrus"
	logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
)

var Logger = logrus.New() //nolint:gochecknoglobals

func LogSetup(debugFlag bool) {
	if debugFlag {
		Logger.SetLevel(logrus.DebugLevel)
	}
}

func LogSyslogSetup() {
	priority := syslog.LOG_EMERG | syslog.LOG_ALERT | syslog.LOG_CRIT |
		syslog.LOG_ERR | syslog.LOG_WARNING | syslog.LOG_NOTICE |
		syslog.LOG_INFO | syslog.LOG_DEBUG
	hook, err := logrus_syslog.NewSyslogHook("", "", priority, "upd")
	if err != nil {
		Logger.WithField("err", err).Error("Unable to use syslog")
	} else {
		Logger.AddHook(hook)
	}
}
