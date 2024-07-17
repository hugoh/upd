package internal

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/shlex"
	"github.com/hugoh/upd/pkg/conncheck"
	"github.com/kr/pretty"
	"github.com/sirupsen/logrus"
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
		var notFoundError *viper.ConfigFileNotFoundError
		if errors.As(err, &notFoundError) {
			return fmt.Errorf("fatal error config file not found: %w", err)
		}
		return fmt.Errorf("fatal error config file: %w", err)
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
	fmt.Printf("%# v\n", pretty.Formatter(loop)) //nolint:forbidigo
}

func getTimeFromConf(key string, unit time.Duration) time.Duration {
	return time.Duration(viper.GetInt(key)) * unit
}

func GetChecksFromConf() ([]*conncheck.Check, error) {
	var checks []*conncheck.Check //nolint:prealloc
	timeout := getTimeFromConf("checks.timeoutMilli", time.Millisecond)
	for _, target := range viper.GetStringSlice("checks.list") {
		url, err := url.Parse(target)
		if err != nil {
			return nil, fmt.Errorf("could not parse check '%s': %w", target, err)
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
	if viper.Get("downAction") == nil {
		return nil, nil //nolint:nilnil
	}
	command, err := shlex.Split(viper.GetString("downAction.exec"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse DownAction definition: %w", err)
	}
	return &DownAction{ //nolint:exhaustruct
		After:    getTimeFromConf("downAction.afterSec", time.Second),
		Every:    getTimeFromConf("downAction.repeatEvery", time.Second),
		Exec:     command[0],
		ExecArgs: command[1:],
	}, nil
}

func GetDelaysFromConf() (map[bool]time.Duration, error) {
	delays := make(map[bool]time.Duration)
	delays[true] = getTimeFromConf("checks.everySec.normal", time.Second)
	delays[false] = getTimeFromConf("checks.everySec.down", time.Second)
	return delays, nil
}
