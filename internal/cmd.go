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
	// ExitCodeError is the exit code for errors.
	ExitCodeError = 1
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
	// ConfigDump is the dump flag name.
	ConfigDump string = "dump"
)

// SetupLoop initializes the loop with configuration from the given file.
func SetupLoop(loop *logic.Loop, configPath string) (*Configuration, error) {
	newConf, err := ReadConf(configPath)
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
		newConf.Stats.Retention,
		&newConf.Stats)

	return newConf, nil
}

// Run is the main application entry point handling signals and configuration reload.
func Run(appCtx context.Context, cmd *cli.Command) error {
	logger.LogSetup(cmd.Bool(ConfigDebug))

	rootCtx, stopSignalHandlers := signal.NotifyContext(appCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignalHandlers()

	sighupCh := make(chan os.Signal, SighupChanSize)
	signal.Notify(sighupCh, syscall.SIGHUP)

	loop := logic.NewLoop()

	for {
		currentWorkerCtx, cancelCurrentWorker := context.WithCancel(rootCtx)

		errCh := make(chan error, ErrChanSize)
		done := make(chan struct{})

		go func(ctx context.Context) {
			defer close(done)

			var err error

			_, err = SetupLoop(loop, cmd.String(ConfigConfig))
			if err != nil {
				errCh <- fmt.Errorf("cannot configure app: %w", err)

				return
			}

			errCh <- nil

			loop.Run(ctx)
			loop.Stop(ctx)
		}(currentWorkerCtx)

		select {
		case <-rootCtx.Done():
			logger.L.Info("[App] shutting down")
			cancelCurrentWorker()
			<-done

			return nil
		case err := <-errCh:
			if err != nil {
				cancelCurrentWorker()
				<-done

				return err
			}

			<-done
		case <-sighupCh:
			logger.L.Info("[App] SIGHUP received: reloading configuration")
			cancelCurrentWorker()
			<-done
		}
	}
}

// Cmd creates and runs the CLI application.
func Cmd() error {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:      ConfigConfig,
			Aliases:   []string{"c"},
			Usage:     "use the specified YAML configuration file",
			Value:     DefaultConfig,
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
