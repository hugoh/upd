// Package internal provides internal configuration, command, and logic
// handling for the upd application.
//
// Configuration:
//
// The Configuration struct is loaded from YAML files and contains all
// settings for the application including:
// - Network connectivity checks (HTTP, TCP, DNS)
// - Check intervals (normal and down states)
// - Down actions to execute when connection fails
// - Statistics server configuration
// - Logging configuration
//
// Example configuration:
//
//	checks:
//	 every:
//	   normal: 2m
//	   down: 30s
//	 list:
//	   ordered:
//	     - http://captive.apple.com/hotspot-detect.html
//	   shuffled:
//	     - dns://8.8.8.8/example.com
//	 timeout: 10s
//	downAction:
//	 exec: "echo 'Connection down'"
//	 every:
//	   after: 60s
//	   repeat: 300s
//	stats:
//	 port: 8080
//	logLevel: debug
//
// Configuration values support environment variable substitution using
// the ${VAR} syntax in YAML string values:
//
//	checks:
//	 timeout: ${UPD_TIMEOUT} // Set UPD_TIMEOUT env var to override
//
// Command Handling:
//
// The Cmd function provides the main CLI interface using the urfave/cli
// library. It supports:
// - Configuration file specification via `-c` or `--config` flag
// - Debug logging via `-d` or `--debug` flag
// - SIGHUP signal handling for configuration reload
// - Graceful shutdown on SIGINT or SIGTERM
//
// Network Security:
//
// Configuration paths are sanitized to prevent path traversal attacks.
// Command execution in DownActions is validated to prevent injection.
package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/hugoh/upd/internal/check"
	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/hugoh/upd/internal/status"
	"github.com/hugoh/upd/internal/types"
	"github.com/pelletier/go-toml/v2"
)

const (
	// DefaultConfig is the default configuration file name.
	DefaultConfig = ".upd.toml"
	// DefaultDNSPort is the default DNS port.
	DefaultDNSPort = "53"

	logLevelTrace = "trace"
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
)

// Configuration holds all application settings.
type Configuration struct {
	Checks struct {
		Every struct {
			Normal types.Duration `toml:"normal"`
			Down   types.Duration `toml:"down"`
		} `toml:"every"`
		List struct {
			Ordered  []string `toml:"ordered"`
			Shuffled []string `toml:"shuffled"`
		} `toml:"list"`
		TimeOut types.Duration `toml:"timeout"`
	} `toml:"checks"`
	DownAction struct {
		Exec  string `toml:"exec"`
		Every struct {
			After        types.Duration `toml:"after"`
			Repeat       types.Duration `toml:"repeat"`
			BackoffLimit types.Duration `toml:"expBackoffLimit"`
		} `toml:"every"`
		StopExec string `toml:"stopExec"`
	} `toml:"downAction"`
	Stats struct {
		Port         int              `toml:"port"`
		Reports      []types.Duration `toml:"reports"`
		Retention    types.Duration   `toml:"retention"`
		ReadTimeout  types.Duration   `toml:"readTimeout"`
		WriteTimeout types.Duration   `toml:"writeTimeout"`
		IdleTimeout  types.Duration   `toml:"idleTimeout"`
	} `toml:"stats"`
	LogLevel string `toml:"logLevel"`
}

func configError(msg string, path string, err error) (*Configuration, error) {
	logger.L.Error(msg, logger.LogComponent, logger.LogComponentConfig, "file", path, "error", err)

	return nil, fmt.Errorf("%s: %w", msg, err)
}

// ReadConf loads and validates configuration from the given file.
func ReadConf(cfgFile string) (*Configuration, error) {
	if cfgFile == "" {
		cfgFile = DefaultConfig
	}

	absPath, err := filepath.Abs(filepath.Clean(cfgFile))
	if err != nil {
		return configError("could not resolve config path", cfgFile, err)
	}

	// Read file - cfgFile has been cleaned and resolved to absolute path
	// #nosec G304 // Path is sanitized by filepath.Abs() and filepath.Clean(), and only reads admin-configured files
	content, err := os.ReadFile(absPath) // #nosec G304
	if err != nil {
		return configError("Could not read config", absPath, err)
	}

	logger.L.Debug(
		"config file used",
		logger.LogComponent,
		logger.LogComponentConfig,
		"file",
		absPath,
	)

	content, err = expandEnvVars(content)
	if err != nil {
		return configError("Unable to parse the config", absPath, err)
	}

	var conf Configuration

	err = toml.Unmarshal(content, &conf)
	if err != nil {
		return configError("Invalid TOML", absPath, err)
	}

	if err := conf.Validate(); err != nil {
		return configError("Missing required attributes", cfgFile, err)
	}

	conf.logSetup()

	return &conf, nil
}

// envVarRe matches ${VAR} patterns for environment variable expansion.
var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

var errMissingEnvVar = errors.New("environment variable is not set")

// expandEnvVars replaces ${VAR} references in raw config bytes. Only braced
// syntax is expanded; $VAR (unbraced) is left unchanged to prevent accidental
// expansion of $PATH, $HOME, etc. Returns an error if any referenced
// environment variable is not set.
func expandEnvVars(content []byte) ([]byte, error) {
	matches := envVarRe.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if _, ok := os.LookupEnv(string(m[1])); !ok {
			return nil, fmt.Errorf("%q: %w", string(m[1]), errMissingEnvVar)
		}
	}

	return envVarRe.ReplaceAllFunc(content, func(match []byte) []byte {
		return []byte(os.Getenv(string(match[2 : len(match)-1])))
	}), nil
}

// ErrNoChecks is returned when no valid checks are found in configuration.
var ErrNoChecks = errors.New("no valid checks found in config")

// GetChecks builds a CheckList from the configuration.
func (c Configuration) GetChecks() (*check.List, error) {
	checkList := &check.List{
		Ordered:  c.GetChecksCat(c.Checks.List.Ordered),
		Shuffled: c.GetChecksCat(c.Checks.List.Shuffled),
	}
	if len(checkList.Ordered) == 0 && len(checkList.Shuffled) == 0 {
		return nil, ErrNoChecks
	}

	return checkList, nil
}

// GetChecksCat creates checks from a list of check URIs.
func (c Configuration) GetChecksCat(category []string) []*check.Check {
	checks := make([]*check.Check, 0, len(category))
	for _, checkStr := range category {
		parsedURL, err := url.Parse(checkStr)
		if err != nil {
			logger.L.Error("could not parse check in config",
				logger.LogComponent, logger.LogComponentConfig, "check", checkStr, "error", err)

			continue
		}

		var probe check.Probe

		switch parsedURL.Scheme {
		case check.DNS:
			domain := strings.TrimPrefix(parsedURL.Path, "/")
			if domain == "" {
				logger.L.Error(
					"DNS check missing domain",
					logger.LogComponent,
					logger.LogComponentConfig,
					"check",
					checkStr,
				)

				continue
			}

			if parsedURL.Host == "" {
				logger.L.Error("DNS check missing resolver host",
					logger.LogComponent, logger.LogComponentConfig, "check", checkStr)

				continue
			}

			port := parsedURL.Port()
			if port == "" {
				port = DefaultDNSPort
			}

			dnsResolver := parsedURL.Host + ":" + port
			probe = check.NewDNSProbe(dnsResolver, domain)
		case check.HTTP, check.HTTPS:
			probe = check.NewHTTPProbe(parsedURL.String())
		case check.TCP:
			hostPort := net.JoinHostPort(parsedURL.Hostname(), parsedURL.Port())
			probe = check.NewTCPProbe(hostPort)
		default:
			logger.L.Error("unknown protocol in config",
				logger.LogComponent, logger.LogComponentConfig, "check", checkStr,
				"protocol", parsedURL.Scheme)

			continue
		}

		checks = append(checks, &check.Check{
			Probe:   probe,
			Timeout: c.Checks.TimeOut.StdDuration(),
		})
	}

	return checks
}

// GetDownAction creates a DownAction from the configuration.
func (c Configuration) GetDownAction() *logic.DownAction {
	if reflect.ValueOf(c.DownAction).IsZero() {
		return nil
	}

	return &logic.DownAction{
		After:        c.DownAction.Every.After.StdDuration(),
		Every:        c.DownAction.Every.Repeat.StdDuration(),
		BackoffLimit: c.DownAction.Every.BackoffLimit.StdDuration(),
		Exec:         c.DownAction.Exec,
		StopExec:     c.DownAction.StopExec,
	}
}

// GetDelays returns the check intervals for up and down states.
func (c Configuration) GetDelays() map[bool]time.Duration {
	delays := make(map[bool]time.Duration)
	delays[true] = c.Checks.Every.Normal.StdDuration()
	delays[false] = c.Checks.Every.Down.StdDuration()

	return delays
}

// GetStatServerConfig creates a runtime stats server config from the TOML-deserialized config.
func (c Configuration) GetStatServerConfig() *status.StatServerConfig {
	reports := make([]time.Duration, len(c.Stats.Reports))
	for i, d := range c.Stats.Reports {
		reports[i] = d.StdDuration()
	}

	return &status.StatServerConfig{
		Port:         c.Stats.Port,
		Reports:      reports,
		Retention:    c.Stats.Retention.StdDuration(),
		ReadTimeout:  c.Stats.ReadTimeout.StdDuration(),
		WriteTimeout: c.Stats.WriteTimeout.StdDuration(),
		IdleTimeout:  c.Stats.IdleTimeout.StdDuration(),
	}
}

func (c Configuration) logSetup() {
	var level slog.Level

	switch c.LogLevel {
	case "":
		level = slog.LevelInfo
	case logLevelTrace:
		level = slog.LevelDebug - 4 //nolint:mnd // slog doesn't have LevelTrace, use LevelDebug - 4
	case logLevelDebug:
		level = slog.LevelDebug
	case logLevelInfo:
		level = slog.LevelInfo
	case logLevelWarn:
		level = slog.LevelWarn
	default:
		logger.L.Error(
			"unknown loglevel",
			logger.LogComponent,
			logger.LogComponentConfig,
			"loglevel",
			c.LogLevel,
		)

		return
	}

	logger.L = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}
