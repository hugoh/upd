// Package internal provides internal configuration, command, and logic handling.
/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hugoh/upd/internal/config"
	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/hugoh/upd/internal/version"
	"github.com/urfave/cli/v3"
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

	loop.Configure(checklist,
		newConf.GetDelays(),
		newConf.GetDownAction(),
		newConf.GetStatServerConfig().Reports...)

	return newConf, nil
}

// Run is the main application entry point handling signals and configuration reload.
func Run(appCtx context.Context, cmd *cli.Command) error {
	logger.LogSetup(cmd.Bool(ConfigDebug))

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

			newConf, err := SetupLoop(loop, cmd.String(ConfigConfig))
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

// Cmd creates and runs the CLI application.
func Cmd() error {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:      ConfigConfig,
			Aliases:   []string{"c"},
			Usage:     "use the specified TOML configuration file",
			Value:     config.DefaultConfig,
			TakesFile: true,
		},
		&cli.BoolFlag{
			Name:    ConfigDebug,
			Aliases: []string{"d"},
			Value:   false,
			Usage:   "display debugging output in the console",
		},
	}

	app := &cli.Command{
		Name:    AppName,
		Usage:   AppShort,
		Version: version.Version(),
		Flags:   flags,
		Action:  Run,
	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		return fmt.Errorf("failed to run app: %w", err)
	}

	return nil
}
