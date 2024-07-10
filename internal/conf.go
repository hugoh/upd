package internal

import (
	"fmt"
	"net/url"
	"time"

	"github.com/hugoh/upd/pkg/conncheck"
	"github.com/sirupsen/logrus"

	"github.com/kr/pretty"
	"github.com/spf13/viper"
)

func ReadConf(cfgFile string) error {
	viper.SetConfigType("yaml")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName(".upd")
		viper.AddConfigPath("$HOME/")
		viper.AddConfigPath(".")
	}

	logrus.WithField("file", viper.ConfigFileUsed()).Debug("[Config] File")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return fmt.Errorf("fatal error config file not found: %w", err)
		} else {
			return fmt.Errorf("fatal error config file: %w", err)
		}
	}

	return nil
}

func LogSetup(debugFlag bool) {
	if debugFlag {
		viper.Set("logLevel", "debug")
	}
	switch l := viper.GetString("logLevel"); l {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	default:
		logrus.WithField("loglevel", l).Error("[Config] Unknown loglevel")
	}
}

func DumpConf(loop *Loop) {
	fmt.Printf("%# v\n", pretty.Formatter(loop))
}

func getTimeFromConf(key string, unit time.Duration) time.Duration {
	return time.Duration(viper.GetInt(key)) * unit
}

func GetChecksFromConf() ([]*conncheck.Check, error) {
	var checks []*conncheck.Check
	timeout := getTimeFromConf("checks.timeoutMilli", time.Millisecond)
	for _, target := range viper.GetStringSlice("checks.list") {
		url, err := url.Parse(target)
		if err != nil {
			return nil, fmt.Errorf(
				"could not parse check '%s': %v",
				target,
				err,
			)
		}
		p := ProtocolByID(url.Scheme)
		if p == nil {
			return nil, fmt.Errorf(
				"unknown protocol '%s' for '%s'",
				url.Scheme,
				target,
			)
		}
		checks = append(checks, &conncheck.Check{
			Proto:   p,
			Target:  target,
			Timeout: timeout,
		})
	}
	return checks, nil
}

func GetDownActionFromConf() (*DownAction, error) {
	return &DownAction{
		After: getTimeFromConf("downAction.afterSec", time.Second),
		Every: getTimeFromConf("downAction.repeatEvery", time.Second),
		Exec:  viper.GetString("downAction.exec"),
	}, nil
}

func GetDelaysFromConf() (map[bool]time.Duration, error) {
	delays := make(map[bool]time.Duration)
	delays[true] = getTimeFromConf("checks.everySec.normal", time.Second)
	delays[false] = getTimeFromConf("checks.everySec.down", time.Second)
	return delays, nil
}
