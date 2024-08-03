package internal

import (
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/go-playground/validator"
	"github.com/hugoh/upd/pkg/conncheck"
	"github.com/kr/pretty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Configuration struct {
	Checks struct {
		Every struct {
			Normal int `mapstructure:"normal" validate:"required,gt=0"`
			Down   int `mapstructure:"down"   validate:"required,gt=0"`
		} `mapstructure:"everySec"`
		List     []string `mapstructure:"list"         validate:"required"`
		TimeOut  int      `mapstructure:"timeoutMilli" validate:"required"`
		Shuffled bool     `mapstructure:"shuffled"`
	} `mapstructure:"checks" validate:"required"`
	DownAction struct {
		Exec  string `mapstructure:"exec" validate:"omitempty"`
		Every struct {
			After        int `mapstructure:"after"           validate:"omitempty,gte=0"`
			Repeat       int `mapstructure:"repeat"          validate:"omitempty,gte=0"`
			BackoffLimit int `mapstructure:"expBackoffLimit" validate:"omitempty,gte=0"`
		} `mapstructure:"everySec"`
		StopExec string `mapstructure:"stopExec" validate:"omitempty"`
	} `mapstructure:"downAction"`
	LogLevel string `mapstructure:"logLevel" validate:"omitempty,oneof=debug info warn"`
}

func configFatal(msg string, err error) {
	logrus.WithFields(logrus.Fields{
		"file": viper.ConfigFileUsed(),
		"err":  err,
	}).Fatal(msg)
}

func ReadConf(cfgFile string) *Configuration {
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
		configFatal("Could not read config", err)
	}
	var conf Configuration
	if err := viper.Unmarshal(&conf); err != nil {
		configFatal("Unable to parse the config", err)
	}
	validate := validator.New()
	if err := validate.Struct(&conf); err != nil {
		configFatal("Missing required attributes", err)
	}

	return &conf
}

func (c *Configuration) LogSetup(debugFlag bool) {
	if debugFlag {
		c.LogLevel = "debug"
	}
	switch c.LogLevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn", "":
		logrus.SetLevel(logrus.WarnLevel)
	default:
		logrus.WithField("loglevel", c.LogLevel).Error("[Config] Unknown loglevel")
	}
}

func (c *Configuration) Dump() {
	fmt.Printf("%# v\n", pretty.Formatter(c)) //nolint:forbidigo
}

func (c *Configuration) GetChecks() []*conncheck.Check {
	var checks []*conncheck.Check //nolint:prealloc
	timeout := time.Duration(c.Checks.TimeOut) * time.Millisecond
	for _, check := range c.Checks.List {
		url, err := url.Parse(check)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"check": check,
				"err":   err,
			}).Fatal("Could not parse check")
		}
		p := ProtocolByID(url.Scheme)
		if p == nil {
			logrus.WithFields(logrus.Fields{
				"check":    check,
				"protocal": url.Scheme,
				"err":      err,
			}).Fatal("Unknown protocol")
		}
		var target string
		switch p.ID {
		case "dns":
			target = url.Hostname()
		case "http":
			target = url.String()
		case "tcp":
			target = fmt.Sprintf("%s:%s", url.Hostname(), url.Port())
		}
		checks = append(checks, &conncheck.Check{
			Proto:   p,
			Target:  target,
			Timeout: timeout,
		})
	}
	return checks
}

func (c *Configuration) GetDownAction() *DownAction {
	if reflect.ValueOf(c.DownAction).IsZero() {
		return nil
	}
	return &DownAction{
		After:        time.Duration(c.DownAction.Every.After) * time.Second,
		Every:        time.Duration(c.DownAction.Every.Repeat) * time.Second,
		BackoffLimit: time.Duration(c.DownAction.Every.BackoffLimit) * time.Second,
		Exec:         c.DownAction.Exec,
		StopExec:     c.DownAction.StopExec,
	}
}

func (c *Configuration) GetDelays() map[bool]time.Duration {
	delays := make(map[bool]time.Duration)
	delays[true] = time.Duration(c.Checks.Every.Normal) * time.Second
	delays[false] = time.Duration(c.Checks.Every.Down) * time.Second
	return delays
}
