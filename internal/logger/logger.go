// Package logger provides the global logger for the application.
package logger

import (
	"log/slog"
	"os"
)

const logComponent = "component"

const (
	logComponentCheck      = "check"
	logComponentDownAction = "downaction"
	logComponentLoop       = "loop"
	logComponentStats      = "stats"
	logComponentConfig     = "config"
	logComponentApp        = "app"
)

// Component returns a logger pre-configured with the given component attribute.
func Component(name string) *slog.Logger { return L.With(logComponent, name) }

// Check returns a logger for the check component.
func Check() *slog.Logger { return Component(logComponentCheck) }

// DownAction returns a logger for the down action component.
func DownAction() *slog.Logger { return Component(logComponentDownAction) }

// Loop returns a logger for the loop component.
func Loop() *slog.Logger { return Component(logComponentLoop) }

// Stats returns a logger for the stats component.
func Stats() *slog.Logger { return Component(logComponentStats) }

// Config returns a logger for the config component.
func Config() *slog.Logger { return Component(logComponentConfig) }

// App returns a logger for the app component.
func App() *slog.Logger { return Component(logComponentApp) }

// L is the global logger instance for the application.
//
//nolint:gochecknoglobals,varnamelen // Standard pattern for application-wide logger
var L *slog.Logger

func init() { //nolint:gochecknoinits // Required for global logger initialization
	L = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// LogSetup configures the logger based on the debug flag.
func LogSetup(debugFlag bool) {
	if debugFlag {
		SetLevel(slog.LevelDebug)
		slog.SetDefault(L)
	}
}

// SetLevel sets the log level of the global logger.
func SetLevel(level slog.Level) {
	L = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}
