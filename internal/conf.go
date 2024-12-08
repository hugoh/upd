package internal

import (
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/go-playground/validator"
	"github.com/hugoh/upd/pkg"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
)

var ConfigFileUsed string //nolint:gochecknoglobals

const (
	DefaultConfig string = ".upd.yaml"
)

type Configuration struct {
	Checks struct {
		Every struct {
			Normal time.Duration `validate:"required,gt=0"`
			Down   time.Duration `validate:"required,gt=0"`
		}
		List     []string      `validate:"required,dive,required,uri"`
		TimeOut  time.Duration `validate:"required,gt=0"`
		Shuffled bool
	} `validate:"required"`
	DownAction struct {
		Exec  string
		Every struct {
			After        time.Duration `validate:"omitempty,gt=0"`
			Repeat       time.Duration `validate:"omitempty,gt=0"`
			BackoffLimit time.Duration `koanf:"expBackoffLimit"   validate:"omitempty,gte=0"`
		}
		StopExec string `validate:"omitempty"`
	} `validate:"omitempty"`
	Stats    StatServerConfig `validate:"omitempty"`
	LogLevel string           `validate:"omitempty,oneof=trace debug info warn"`
}

func configFatal(msg string, path string, err error) {
	logrus.WithField("file", path).WithError(err).Fatal(msg)
}

func ReadConf(cfgFile string, printConfig bool) *Configuration {
	k := koanf.New(".")

	if cfgFile == "" {
		cfgFile = DefaultConfig
	}
	if err := k.Load(file.Provider(cfgFile), yaml.Parser()); err != nil {
		configFatal("Could not read config", cfgFile, err)
	}
	logger.WithField("file", cfgFile).Debug("[Config] config file used")
	var conf Configuration
	if err := k.UnmarshalWithConf("", &conf, koanf.UnmarshalConf{}); err != nil {
		configFatal("Unable to parse the config", cfgFile, err)
	}

	if printConfig {
		k.Print()
	}

	validate := validator.New()
	if err := validate.RegisterValidation("validTCPPort", isValidTCPPort); err != nil {
		logrus.WithError(err).Fatal("failed to instantiate config validator")
	}
	if err := validate.Struct(&conf); err != nil {
		configFatal("Missing required attributes", cfgFile, err)
	}

	conf.logSetup()
	return &conf
}

func (c Configuration) logSetup() {
	if logger.GetLevel() == logrus.DebugLevel {
		// Already set
		return
	}
	switch c.LogLevel {
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn", "":
		logger.SetLevel(logrus.WarnLevel)
	default:
		logger.WithField("loglevel", c.LogLevel).Error("[Config] Unknown loglevel")
	}
}

func (c Configuration) GetChecks() []*pkg.Check {
	checks := make([]*pkg.Check, 0, len(c.Checks.List))
	for _, check := range c.Checks.List {
		url, err := url.Parse(check)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"check": check,
				"err":   err,
			}).Error("could not parse check in config")
			continue
		}
		var probe pkg.Probe
		switch url.Scheme {
		case pkg.DNS:
			domain := url.Path[1:]
			port := url.Port()
			if port == "" {
				port = "53"
			}
			dnsResolver := url.Host + ":" + port
			probe = pkg.GetDNSProbe(dnsResolver, domain)
		case pkg.HTTP, pkg.HTTPS:
			probe = pkg.GetHTTPProbe(url.String())
		case pkg.TCP:
			hostPort := fmt.Sprintf("%s:%s", url.Hostname(), url.Port())
			probe = pkg.GetTCPProbe(hostPort)
		default:
			logger.WithFields(logrus.Fields{
				"check":    check,
				"protocol": url.Scheme,
			}).Error("unknown protocol in config")
			continue
		}
		checks = append(checks, &pkg.Check{
			Probe:   &probe,
			Timeout: c.Checks.TimeOut,
		})
	}
	if len(checks) == 0 {
		logger.Fatal("No valid check found")
	}
	return checks
}

func (c Configuration) GetDownAction() *DownAction {
	if reflect.ValueOf(c.DownAction).IsZero() {
		return nil
	}
	return &DownAction{
		After:        c.DownAction.Every.After,
		Every:        c.DownAction.Every.Repeat,
		BackoffLimit: c.DownAction.Every.BackoffLimit,
		Exec:         c.DownAction.Exec,
		StopExec:     c.DownAction.StopExec,
	}
}

func (c Configuration) GetDelays() map[bool]time.Duration {
	delays := make(map[bool]time.Duration)
	delays[true] = c.Checks.Every.Normal
	delays[false] = c.Checks.Every.Down
	return delays
}
