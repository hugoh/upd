package internal

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"time"

	"github.com/go-playground/validator"
	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/hugoh/upd/internal/status"
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
	Stats    status.StatServerConfig `validate:"omitempty"`
	LogLevel string                  `validate:"omitempty,oneof=trace debug info warn"`
}

func configError(msg string, path string, err error) (*Configuration, error) {
	logrus.WithField("file", path).WithError(err).Error(msg)
	return nil, fmt.Errorf("%s: %w", msg, err)
}

func ReadConf(cfgFile string) (*Configuration, error) {
	k := koanf.New(".")

	if cfgFile == "" {
		cfgFile = DefaultConfig
	}
	if err := k.Load(file.Provider(cfgFile), yaml.Parser()); err != nil {
		return configError("Could not read config", cfgFile, err)
	}
	logger.L.WithField("file", cfgFile).Debug("[Config] config file used")
	var conf Configuration
	if err := k.UnmarshalWithConf("", &conf, koanf.UnmarshalConf{}); err != nil {
		return configError("Unable to parse the config", cfgFile, err)
	}

	validate := validator.New()
	if err := validate.RegisterValidation("validTCPPort", isValidTCPPort); err != nil {
		logrus.WithError(err).Fatal("failed to instantiate config validator")
	}
	if err := validate.Struct(&conf); err != nil {
		return configError("Missing required attributes", cfgFile, err)
	}

	conf.logSetup()
	return &conf, nil
}

func isValidTCPPort(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^:(6553[0-5]|655[0-2][0-9]|65[0-4][0-9]{2}|6[0-4][0-9]{3}|[1-5][0-9]{4}|[0-9]{1,4})$`)
	return re.MatchString(fl.Field().String())
}

var ErrNoChecks = errors.New("no valid checks found in config")

func (c Configuration) GetChecks() ([]*pkg.Check, error) {
	checks := make([]*pkg.Check, 0, len(c.Checks.List))
	for _, check := range c.Checks.List {
		url, err := url.Parse(check)
		if err != nil {
			logger.L.WithFields(logrus.Fields{
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
			logger.L.WithFields(logrus.Fields{
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
		return nil, ErrNoChecks
	}
	return checks, nil
}

func (c Configuration) GetDownAction() *logic.DownAction {
	if reflect.ValueOf(c.DownAction).IsZero() {
		return nil
	}
	return &logic.DownAction{
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

func (c Configuration) logSetup() {
	if logger.L.GetLevel() == logrus.DebugLevel {
		// Already set
		return
	}
	switch c.LogLevel {
	case "trace":
		logger.L.SetLevel(logrus.TraceLevel)
	case "debug":
		logger.L.SetLevel(logrus.DebugLevel)
	case "info":
		logger.L.SetLevel(logrus.InfoLevel)
	case "warn", "":
		logger.L.SetLevel(logrus.WarnLevel)
	default:
		logger.L.WithField("loglevel", c.LogLevel).Error("[Config] Unknown loglevel")
	}
}
