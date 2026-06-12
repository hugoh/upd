// Package config provides configuration loading, validation, and factory methods
// for the upd application.
//
// Configuration:
//
// The Configuration struct is loaded from TOML files and contains all
// settings for the application including:
// - Network connectivity checks (HTTP, TCP, DNS)
// - Check intervals (normal and down states)
// - Down actions to execute when connection fails
// - Statistics server configuration
// - Logging configuration
//
// Example configuration:
//
//	logLevel = "debug"
//
//	[checks]
//	timeout = "10s"
//
//	[checks.every]
//	normal = "2m"
//	down = "30s"
//
//	[checks.list]
//	ordered = ["http://captive.apple.com/hotspot-detect.html"]
//	shuffled = ["dns://8.8.8.8/example.com"]
//
//	[downAction]
//	exec = "echo 'Connection down'"
//
//	[downAction.every]
//	after = "60s"
//	repeat = "300s"
//
//	[stats]
//	port = 8080
//
// Configuration values support environment variable substitution using
// the ${VAR} syntax in TOML string values:
//
//	[checks]
//	timeout = "${UPD_TIMEOUT}" # Set UPD_TIMEOUT env var to override
//
// Network Security:
//
// Configuration paths are sanitized to prevent path traversal attacks.
// Command execution in DownActions is validated to prevent injection.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hugoh/upd/internal/check"
	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/hugoh/upd/internal/status"
	"github.com/pelletier/go-toml/v2"
)

const (
	// DefaultConfig is the default configuration file name.
	DefaultConfig = ".upd.toml"
)

// ChecksEveryConfig holds the check intervals for up and down states.
type ChecksEveryConfig struct {
	Normal Duration `toml:"normal"`
	Down   Duration `toml:"down"`
}

// ChecksListConfig holds the ordered and shuffled check URI lists.
type ChecksListConfig struct {
	Ordered  []string `toml:"ordered"`
	Shuffled []string `toml:"shuffled"`
}

// ChecksConfig holds the connectivity check settings.
type ChecksConfig struct {
	Every   ChecksEveryConfig `toml:"every"`
	List    ChecksListConfig  `toml:"list"`
	TimeOut Duration          `toml:"timeout"`
}

// DownActionEveryConfig holds the down action scheduling settings.
type DownActionEveryConfig struct {
	After        Duration `toml:"after"`
	Repeat       Duration `toml:"repeat"`
	BackoffLimit Duration `toml:"expBackoffLimit"`
}

// DownActionConfig holds the down action settings.
type DownActionConfig struct {
	Exec     string                `toml:"exec"`
	Every    DownActionEveryConfig `toml:"every"`
	StopExec string                `toml:"stopExec"`
}

// StatsConfig holds the statistics server settings.
type StatsConfig struct {
	Port         int        `toml:"port"`
	Reports      []Duration `toml:"reports"`
	ReadTimeout  Duration   `toml:"readTimeout"`
	WriteTimeout Duration   `toml:"writeTimeout"`
	IdleTimeout  Duration   `toml:"idleTimeout"`
}

// Configuration holds all application settings.
type Configuration struct {
	Checks     ChecksConfig     `toml:"checks"`
	DownAction DownActionConfig `toml:"downAction"`
	Stats      StatsConfig      `toml:"stats"`
	LogLevel   string           `toml:"logLevel"`
}

func configError(msg string, path string, err error) (*Configuration, error) {
	logger.Config().Error(msg, "file", path, "error", err)

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
		return configError("could not read config", absPath, err)
	}

	logger.Config().Debug("config file used", "file", absPath)

	content, err = expandEnvVars(content)
	if err != nil {
		return configError("unable to parse the config", absPath, err)
	}

	var conf Configuration

	err = toml.Unmarshal(content, &conf)
	if err != nil {
		return configError("invalid TOML", absPath, err)
	}

	if err := conf.Validate(); err != nil {
		return configError("missing required attributes", cfgFile, err)
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

// ErrNoChecks is returned when no checks are found in configuration.
var ErrNoChecks = errors.New("no checks found in config")

// GetChecks builds a CheckList from the configuration.
// Any invalid check fails the whole configuration.
func (c Configuration) GetChecks() (*check.List, error) {
	ordered, err := c.GetChecksCat(c.Checks.List.Ordered)
	if err != nil {
		return nil, err
	}

	shuffled, err := c.GetChecksCat(c.Checks.List.Shuffled)
	if err != nil {
		return nil, err
	}

	if len(ordered) == 0 && len(shuffled) == 0 {
		return nil, ErrNoChecks
	}

	return &check.List{Ordered: ordered, Shuffled: shuffled}, nil
}

//nolint:ireturn // intentionally returns interface to abstract probe creation
func probeFromURL(parsedURL *url.URL) (check.Probe, error) {
	switch parsedURL.Scheme {
	case check.DNS:
		probe, err := check.NewDNSProbe(parsedURL.Host, strings.TrimPrefix(parsedURL.Path, "/"))
		if err != nil {
			return nil, fmt.Errorf("invalid DNS check: %w", err)
		}

		return probe, nil
	case check.HTTP, check.HTTPS:
		return check.NewHTTPProbe(parsedURL.String()), nil
	case check.TCP:
		return check.NewTCPProbe(net.JoinHostPort(parsedURL.Hostname(), parsedURL.Port())), nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnsupportedScheme, parsedURL.Scheme)
	}
}

// GetChecksCat creates checks from a list of check URIs.
func (c Configuration) GetChecksCat(category []string) ([]*check.Check, error) {
	checks := make([]*check.Check, 0, len(category))
	for _, checkStr := range category {
		parsedURL, err := url.Parse(checkStr)
		if err != nil {
			return nil, fmt.Errorf("could not parse check %q: %w", checkStr, err)
		}

		probe, err := probeFromURL(parsedURL)
		if err != nil {
			return nil, fmt.Errorf("check %q: %w", checkStr, err)
		}

		checks = append(checks, &check.Check{
			Probe:   probe,
			Timeout: c.Checks.TimeOut.StdDuration(),
		})
	}

	return checks, nil
}

// GetDownAction creates a DownAction from the configuration.
func (c Configuration) GetDownAction() *logic.DownAction {
	if c.DownAction == (DownActionConfig{}) {
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
func (c Configuration) GetDelays() logic.Delays {
	return logic.Delays{
		Up:   c.Checks.Every.Normal.StdDuration(),
		Down: c.Checks.Every.Down.StdDuration(),
	}
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
		ReadTimeout:  c.Stats.ReadTimeout.StdDuration(),
		WriteTimeout: c.Stats.WriteTimeout.StdDuration(),
		IdleTimeout:  c.Stats.IdleTimeout.StdDuration(),
	}
}

func (c Configuration) logSetup() {
	if c.LogLevel == "" {
		return
	}

	var level slog.Level

	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		logger.Config().Error("unknown loglevel", "loglevel", c.LogLevel)

		return
	}

	logger.SetLevel(level)
}
