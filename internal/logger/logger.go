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

//nolint:gochecknoglobals // application-wide logger state
var (
	// levelVar allows changing the log level atomically without replacing the
	// logger, so level changes apply to all loggers already handed out.
	levelVar = new(slog.LevelVar)

	// L is the global logger instance for the application.
	//nolint:varnamelen // Standard pattern for application-wide logger
	L = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: levelVar,
	}))

	checkLogger      = Component(logComponentCheck)
	downActionLogger = Component(logComponentDownAction)
	loopLogger       = Component(logComponentLoop)
	statsLogger      = Component(logComponentStats)
	configLogger     = Component(logComponentConfig)
	appLogger        = Component(logComponentApp)
)

// Component returns a logger pre-configured with the given component attribute.
func Component(name string) *slog.Logger { return L.With(logComponent, name) }

// Check returns a logger for the check component.
func Check() *slog.Logger { return checkLogger }

// DownAction returns a logger for the down action component.
func DownAction() *slog.Logger { return downActionLogger }

// Loop returns a logger for the loop component.
func Loop() *slog.Logger { return loopLogger }

// Stats returns a logger for the stats component.
func Stats() *slog.Logger { return statsLogger }

// Config returns a logger for the config component.
func Config() *slog.Logger { return configLogger }

// App returns a logger for the app component.
func App() *slog.Logger { return appLogger }

// LogSetup configures the logger based on the debug flag.
func LogSetup(debugFlag bool) {
	if debugFlag {
		SetLevel(slog.LevelDebug)
	}

	slog.SetDefault(L)
}

// SetLevel sets the log level of the global logger.
func SetLevel(level slog.Level) {
	levelVar.Set(level)
}
