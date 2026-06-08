// Package logger provides the global logger for the application.
package logger

import (
	"log/slog"
	"os"
)

// LogComponent is the structured log key used for component identification.
const LogComponent = "component"

// Component values used across the codebase for log attribution.
const (
	LogComponentCheck      = "check"
	LogComponentDownAction = "downaction"
	LogComponentLoop       = "loop"
	LogComponentStats      = "stats"
	LogComponentConfig     = "config"
	LogComponentApp        = "app"
)

// Component returns a logger pre-configured with the given component attribute.
func Component(name string) *slog.Logger { return L.With(LogComponent, name) }

// Check returns a logger for the check component.
func Check() *slog.Logger { return Component(LogComponentCheck) }

// DownAction returns a logger for the down action component.
func DownAction() *slog.Logger { return Component(LogComponentDownAction) }

// Loop returns a logger for the loop component.
func Loop() *slog.Logger { return Component(LogComponentLoop) }

// Stats returns a logger for the stats component.
func Stats() *slog.Logger { return Component(LogComponentStats) }

// Config returns a logger for the config component.
func Config() *slog.Logger { return Component(LogComponentConfig) }

// App returns a logger for the app component.
func App() *slog.Logger { return Component(LogComponentApp) }

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
		L = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		slog.SetDefault(L)
	}
}
