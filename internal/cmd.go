/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/urfave/cli/v3"
)

const (
	AppName  = "upd"
	AppShort = "Tool to monitor if the network connection is up."
)

const (
	ConfigConfig string = "config"
	ConfigDebug  string = "debug"
	ConfigDump   string = "dump"
)

func SetupLoop(loop *logic.Loop, conf *Configuration, configPath string) error {
	newConf, err := ReadConf(configPath)
	if err != nil {
		if conf == nil {
			logger.L.WithError(err).Error("error reading configuration")
			// This will never be reached because of the fatal log
			return fmt.Errorf("error reading configuration: %w", err)
		}
		logger.L.Error("[App] reusing previous configuration")
		return nil
	}
	conf = newConf
	loop.Configure(conf.GetChecks(),
		conf.GetDelays(),
		conf.GetDownAction(),
		conf.Checks.Shuffled,
		conf.Stats.Retention,
		&conf.Stats)
	return nil
}

func Run(appCtx context.Context, cmd *cli.Command) error {
	logger.LogSetup(cmd.Bool(ConfigDebug))

	rootCtx, stopSignalHandlers := signal.NotifyContext(appCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignalHandlers()

	sighupCh := make(chan os.Signal, 1)
	signal.Notify(sighupCh, syscall.SIGHUP)

	loop := logic.NewLoop(cmd.Version)

	var conf *Configuration
	for {
		currentWorkerCtx, cancelCurrentWorker := context.WithCancel(rootCtx)

		// Run the loop in a goroutine so we can cancel it from outside
		done := make(chan struct{})
		go func(ctx context.Context) {
			if err := SetupLoop(loop, conf, cmd.String(ConfigConfig)); err != nil {
				logger.L.Fatal("cannot configure app")
			}
			loop.Run(ctx)
			loop.Stop(ctx)
			close(done)
		}(currentWorkerCtx)

		select {
		case <-rootCtx.Done():
			// Program is terminating
			logger.L.Info("[App] shutting down")
			cancelCurrentWorker()
			<-done
			return nil
		case <-sighupCh:
			// SIGHUP received: restart the loop
			logger.L.Info("[App] SIGHUP received: reloading configuration")
			cancelCurrentWorker()
			<-done
		}
	}
}

func Cmd(version string) {
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
		Version: version,
		Flags:   flags,
		Action:  Run,
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
