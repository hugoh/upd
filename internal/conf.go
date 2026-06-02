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
	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfig is the default configuration file name.
	DefaultConfig = ".upd.yaml"
	// DefaultDNSPort is the default DNS port.
	DefaultDNSPort = "53"
)

// ConfigFileUsed stores the path of the active configuration file for debugging purposes.
//
//nolint:gochecknoglobals // Package-level state for debugging and diagnostics
var ConfigFileUsed string

// Configuration holds all application settings.
type Configuration struct {
	Checks struct {
		Every struct {
			Normal types.Duration
			Down   types.Duration
		}
		List struct {
			Ordered  []string
			Shuffled []string
		}
		TimeOut types.Duration
	}
	DownAction struct {
		Exec  string
		Every struct {
			After        types.Duration
			Repeat       types.Duration
			BackoffLimit types.Duration `yaml:"expBackoffLimit"`
		}
		StopExec string `yaml:"stopExec"`
	} `yaml:"downAction"`
	Stats    status.StatServerConfig
	LogLevel string `yaml:"logLevel"`
}

func configError(msg string, path string, err error) (*Configuration, error) {
	slog.Error(msg, "file", path, "error", err)

	return nil, fmt.Errorf("%s: %w", msg, err)
}

// ReadConf loads and validates configuration from the given file.
func ReadConf(cfgFile string) (*Configuration, error) {
	var err error

	if cfgFile == "" {
		cfgFile = DefaultConfig
	}

	absPath, err := filepath.Abs(filepath.Clean(cfgFile))
	if err != nil {
		return configError("could not resolve config path", cfgFile, err)
	}

	// Read file - cfgFile has been cleaned and resolved to absolute path
	// #nosec G304 // Path is sanitized by filepath.Abs() and filepath.Clean(), and only reads admin-configured files
	var content []byte

	content, err = os.ReadFile(absPath) // #nosec G304
	if err != nil {
		return configError("Could not read config", absPath, err)
	}

	logger.L.Debug("[Config] config file used", "file", absPath)

	var doc yaml.Node

	err = yaml.Unmarshal(content, &doc)
	if err != nil {
		return configError("Invalid YAML", absPath, err)
	}

	if err := expandNode(&doc); err != nil {
		return configError("Unable to parse the config", absPath, err)
	}

	var conf Configuration

	err = doc.Decode(&conf)
	if err != nil {
		return configError("Unable to parse the config", cfgFile, err)
	}

	if err := conf.Validate(); err != nil {
		return configError("Missing required attributes", cfgFile, err)
	}

	conf.logSetup()

	return &conf, nil
}

// expandEnvRe matches ${VAR} patterns for environment variable expansion.
var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandNode recursively walks a yaml.Node tree and expands ${VAR} references
// in string scalar values. Only braced syntax is expanded; $VAR (unbraced) is
// left unchanged to prevent accidental expansion of $PATH, $HOME, etc.
// Returns an error if any referenced environment variable is not set.
func expandNode(node *yaml.Node) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.ScalarNode:
		return expandScalarNode(node)

	case yaml.MappingNode, yaml.SequenceNode, yaml.DocumentNode:
		for _, child := range node.Content {
			if err := expandNode(child); err != nil {
				return err
			}
		}

	case yaml.AliasNode:
		return expandNode(node.Alias)
	}

	return nil
}

const strTag = "!!str"

var errMissingEnvVar = errors.New("environment variable is not set")

func expandScalarNode(node *yaml.Node) error {
	if node.Tag != strTag && node.Tag != "" {
		return nil
	}

	matches := envVarRe.FindAllStringSubmatch(node.Value, -1)
	for _, m := range matches {
		if _, ok := os.LookupEnv(m[1]); !ok {
			return fmt.Errorf("%q: %w", m[1], errMissingEnvVar)
		}
	}

	node.Value = envVarRe.ReplaceAllStringFunc(node.Value, func(match string) string {
		return os.Getenv(match[2 : len(match)-1])
	})

	return nil
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
			logger.L.Error("could not parse check in config", "check", checkStr, "error", err)

			continue
		}

		var probe check.Probe

		switch parsedURL.Scheme {
		case check.DNS:
			domain := strings.TrimPrefix(parsedURL.Path, "/")
			if domain == "" {
				logger.L.Error("DNS check missing domain", "check", checkStr)

				continue
			}

			if parsedURL.Host == "" {
				logger.L.Error("DNS check missing resolver host", "check", checkStr)

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
			logger.L.Error(
				"unknown protocol in config",
				"check",
				checkStr,
				"protocol",
				parsedURL.Scheme,
			)

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

func (c Configuration) logSetup() {
	if c.LogLevel == "" {
		return
	}

	var level slog.Level

	switch c.LogLevel {
	case "trace":
		level = slog.LevelDebug - 4 //nolint:mnd // slog doesn't have LevelTrace, use LevelDebug - 4
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	default:
		logger.L.Error("[Config] Unknown loglevel", "loglevel", c.LogLevel)

		return
	}

	logger.L = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}
