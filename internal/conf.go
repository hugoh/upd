package internal

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"time"

	"github.com/drone/envsubst"
	"github.com/go-playground/validator"
	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/hugoh/upd/internal/status"
	"github.com/hugoh/upd/pkg"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
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
		List struct {
			Ordered  []string `validate:"dive,uri"`
			Shuffled []string `validate:"dive,uri"`
		} `validate:"required"`
		TimeOut time.Duration `validate:"required,gt=0"`
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

	// Read file and substitute environment variables using envsubst
	content, err := os.ReadFile(cfgFile)
	if err != nil {
		return configError("Could not read config", cfgFile, err)
	}

	// Use envsubst to substitute environment variables
	substContent, err := envsubst.EvalEnv(string(content))
	if err != nil {
		return configError("envsubst failed", cfgFile, err)
	}

	// Use koanf rawbytes provider to load the substituted config content
	if err := k.Load(rawbytes.Provider([]byte(substContent)), yaml.Parser()); err != nil {
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

func (c Configuration) GetChecks() (*pkg.CheckList, error) {
	checkList := &pkg.CheckList{
		Ordered:  c.GetChecksCat(c.Checks.List.Ordered),
		Shuffled: c.GetChecksCat(c.Checks.List.Shuffled),
	}
	if len(checkList.Ordered) == 0 && len(checkList.Shuffled) == 0 {
		return nil, ErrNoChecks
	}
	return checkList, nil
}

func (c Configuration) GetChecksCat(category []string) []*pkg.Check {
	checks := make([]*pkg.Check, 0, len(category))
	for _, check := range category {
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
			probe = pkg.NewDNSProbe(dnsResolver, domain)
		case pkg.HTTP, pkg.HTTPS:
			probe = pkg.NewHTTPProbe(url.String())
		case pkg.TCP:
			hostPort := fmt.Sprintf("%s:%s", url.Hostname(), url.Port())
			probe = pkg.NewTCPProbe(hostPort)
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
	return checks
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
