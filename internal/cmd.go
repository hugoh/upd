// Package internal provides internal configuration, command, and logic handling.
/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package internal

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hugoh/upd/internal/config"
	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/hugoh/upd/internal/version"
)

const (
	// AppName is the application name.
	AppName = "upd"
	// AppShort is the application short description.
	AppShort = "Tool to monitor if the network connection is up."
	// ErrChanSize is the buffer size for error channels.
	ErrChanSize = 1
	// SighupChanSize is the buffer size for SIGHUP channels.
	SighupChanSize = 1
)

const (
	// ConfigConfig is the config file flag name.
	ConfigConfig string = "config"
	// ConfigDebug is the debug flag name.
	ConfigDebug string = "debug"
)

// Flags holds the parsed command-line flags.
type Flags struct {
	ConfigPath string
	Debug      bool
}

// SetupLoop initializes the loop with configuration from the given file.
func SetupLoop(loop *logic.Loop, configPath string) (*config.Configuration, error) {
	newConf, err := config.ReadConf(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading configuration: %w", err)
	}

	checklist, checkErr := newConf.GetChecks()
	if checkErr != nil {
		return nil, fmt.Errorf("invalid checks in configuration: %w", checkErr)
	}

	statCfg := newConf.GetStatServerConfig()

	loop.Configure(checklist,
		newConf.GetDelays(),
		newConf.GetDownAction(),
		statCfg.Buckets,
		statCfg.Reports...)

	return newConf, nil
}

// Run is the main application entry point handling signals and configuration reload.
func Run(appCtx context.Context, flags Flags) error {
	logger.LogSetup(flags.Debug)

	rootCtx, stopSignalHandlers := signal.NotifyContext(appCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignalHandlers()

	sighupCh := make(chan os.Signal, SighupChanSize)

	signal.Notify(sighupCh, syscall.SIGHUP)
	defer signal.Stop(sighupCh)

	loop := logic.NewLoop()

	for {
		if rootCtx.Err() != nil {
			logger.App().Info("shutting down")

			return nil
		}

		currentWorkerCtx, cancelCurrentWorker := context.WithCancel(rootCtx)

		errCh := make(chan error, ErrChanSize)
		done := make(chan struct{})

		go func(ctx context.Context) {
			defer close(done)

			newConf, err := SetupLoop(loop, flags.ConfigPath)
			if err != nil {
				errCh <- fmt.Errorf("cannot configure app: %w", err)

				return
			}

			errCh <- nil

			loop.Run(ctx, newConf.GetStatServerConfig())
			loop.Stop(ctx)
		}(currentWorkerCtx)

		err := waitForWorker(rootCtx, sighupCh, errCh, done, cancelCurrentWorker)
		if err != nil {
			return err
		}
	}
}

// waitForWorker blocks until the current worker needs to be replaced or the
// application should terminate. It keeps handling shutdown and reload signals
// while the worker is running. A non-nil return value terminates the
// application.
func waitForWorker(
	rootCtx context.Context,
	sighupCh <-chan os.Signal,
	errCh <-chan error,
	done <-chan struct{},
	cancelWorker context.CancelFunc,
) error {
	stopWorker := func() {
		cancelWorker()
		<-done
	}

	for {
		select {
		case <-rootCtx.Done():
			stopWorker()

			return nil
		case err := <-errCh:
			if err != nil {
				stopWorker()

				return err
			}
			// Configuration loaded; keep waiting for signals.
		case <-sighupCh:
			logger.App().Info("SIGHUP received: reloading configuration")
			stopWorker()

			return nil
		case <-done:
			// Worker exited on its own; surface a pending error if any.
			select {
			case err := <-errCh:
				if err != nil {
					return err
				}
			default:
			}

			return nil
		}
	}
}

// ParseFlags parses the given command-line arguments (excluding the program
// name) into Flags. It returns flag.ErrHelp when --help or --version was
// requested and already handled, in which case the caller should exit
// without running the app.
func ParseFlags(args []string) (Flags, error) {
	var (
		flags       Flags
		showVersion bool
	)

	flagSet := flag.NewFlagSet(AppName, flag.ContinueOnError)
	flagSet.Usage = func() {
		usage := fmt.Sprintf(
			"%s - %s\n\nUsage:\n  %s [flags]\n\nFlags:\n",
			AppName,
			AppShort,
			AppName,
		)
		if _, err := fmt.Fprint(flagSet.Output(), usage); err != nil {
			return
		}

		flagSet.PrintDefaults()
	}

	flagSet.StringVar(
		&flags.ConfigPath,
		ConfigConfig,
		config.DefaultConfig,
		"use the specified TOML configuration file",
	)
	flagSet.StringVar(&flags.ConfigPath, "c", config.DefaultConfig, "shorthand for --"+ConfigConfig)
	flagSet.BoolVar(&flags.Debug, ConfigDebug, false, "display debugging output in the console")
	flagSet.BoolVar(&flags.Debug, "d", false, "shorthand for --"+ConfigDebug)
	flagSet.BoolVar(&showVersion, "version", false, "print the version and exit")

	if err := flagSet.Parse(args); err != nil {
		return Flags{}, fmt.Errorf("failed to parse flags: %w", err)
	}

	if showVersion {
		versionLine := fmt.Sprintf("%s version %s\n", AppName, version.Version())
		if _, err := fmt.Fprint(os.Stdout, versionLine); err != nil {
			return Flags{}, fmt.Errorf("failed to print version: %w", err)
		}

		return Flags{}, flag.ErrHelp
	}

	return flags, nil
}

// Cmd creates and runs the CLI application.
func Cmd() error {
	flags, err := ParseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	if err := Run(context.Background(), flags); err != nil {
		return fmt.Errorf("failed to run app: %w", err)
	}

	return nil
}
