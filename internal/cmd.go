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
	"github.com/hugoh/upd/pkg"
	"github.com/urfave/cli/v3"
)

const (
	AppName        = "upd"
	AppShort       = "Tool to monitor if the network connection is up."
	ExitCodeError  = 1
	ErrChanSize    = 1
	SighupChanSize = 1
)

const (
	ConfigConfig string = "config"
	ConfigDebug  string = "debug"
	ConfigDump   string = "dump"
)

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
		case <-sighupCh:
			logger.L.Info("[App] SIGHUP received: reloading configuration")
			cancelCurrentWorker()
			<-done
		}
	}
}

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
		Version: pkg.Version(),
		Flags:   flags,
		Action:  Run,
	}

	return fmt.Errorf("failed to run app: %w", app.Run(context.Background(), os.Args))
}
