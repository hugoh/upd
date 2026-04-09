// Package logger provides the global logger for the application.
package logger

import (
	"log/slog"
	"os"
)

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
