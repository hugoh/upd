// Package internal provides internal configuration, command, and logic
// handling for the upd application.
//
// Configuration:
//
// The Configuration struct is loaded from YAML files and contains all
// settings for the application including:
//   - Network connectivity checks (HTTP, TCP, DNS)
//   - Check intervals (normal and down states)
//   - Down actions to execute when connection fails
//   - Statistics server configuration
//   - Logging configuration
//
// Example configuration:
//
//	checks:
//	  every:
//	    normal: 2m
//	    down: 30s
//	  list:
//	    ordered:
//	      - http://captive.apple.com/hotspot-detect.html
//	    shuffled:
//	      - dns://8.8.8.8/example.com
//	  timeout: 10s
//	downAction:
//	  exec: \"echo 'Connection down'\"
//	  every:
//	    after: 60s
//	    repeat: 300s
//	stats:
//	  port: \":8080\"
//	logLevel: debug
//
// The configuration supports environment variable substitution using
// the ${VAR} or $VAR syntax.
//
// Example with environment variables:
//
//	checks:
//	  timeout: ${UPD_TIMEOUT:10s}  // Use UPD_TIMEOUT env var or 10s default
//
// Command Handling:
//
// The Cmd function provides the main CLI interface using the urfave/cli
// library. It supports:
//   - Configuration file specification via `-c` or `--config` flag
//   - Debug logging via `-d` or `--debug` flag
//   - SIGHUP signal handling for configuration reload
//   - Graceful shutdown on SIGINT or SIGTERM
//
// Network Security:
//
// Configuration paths are sanitized to prevent path traversal attacks.
// Command execution in DownActions is validated to prevent injection.
package internal

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
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
	var err error

	cfg := koanf.New(".")

	if cfgFile == "" {
		cfgFile = DefaultConfig
	}

	absPath, err := filepath.Abs(filepath.Clean(cfgFile))
	if err != nil {
		return configError("could not resolve config path", cfgFile, err)
	}

	// Read file - cfgFile has been cleaned and resolved to absolute path
	var content []byte
	content, err = os.ReadFile(absPath) // #nosec G304 -- path sanitized by filepath.Abs and filepath.Clean
	if err != nil {
		return configError("Could not read config", absPath, err)
	}

	logger.L.WithField("file", absPath).Debug("[Config] config file used")

	substContent, err := envsubst.EvalEnv(string(content))
	if err != nil {
		return configError("envsubst failed", absPath, err)
	}

	// Use koanf rawbytes provider to load the substituted config content
	err = cfg.Load(rawbytes.Provider([]byte(substContent)), yaml.Parser())
	if err != nil {
		return configError("Could not read config", absPath, err)
	}
	var conf Configuration
	err = cfg.UnmarshalWithConf("", &conf, koanf.UnmarshalConf{})
	if err != nil {
		return configError("Unable to parse the config", cfgFile, err)
	}

	validate := validator.New()
	err = validate.RegisterValidation("validTCPPort", isValidTCPPort)
	if err != nil {
		return configError("failed to instantiate config validator", cfgFile, err)
	}
	err = validate.Struct(&conf)
	if err != nil {
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
			domain := strings.TrimPrefix(url.Path, "/")
			if domain == "" {
				logger.L.WithFields(logrus.Fields{
					"check": check,
				}).Error("DNS check missing domain")
				continue
			}
			if url.Host == "" {
				logger.L.WithFields(logrus.Fields{
					"check": check,
				}).Error("DNS check missing resolver host")
				continue
			}
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
