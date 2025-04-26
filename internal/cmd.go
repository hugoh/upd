/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package internal

import (
	"context"
	"log"
	"os"

	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/hugoh/upd/internal/status"
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

func Run(_ context.Context, cCtx *cli.Command) error {
	logger.LogSetup(cCtx.Bool(ConfigDebug))
	dump := cCtx.Bool(ConfigDump)
	conf := ReadConf(cCtx.String(ConfigConfig), dump)

	if dump {
		return nil
	}

	checks := conf.GetChecks()
	delays := conf.GetDelays()
	da := conf.GetDownAction()

	s := status.NewStatus(cCtx.Version, conf.Stats.Retention)

	loop := logic.NewLoop(checks, delays, da, conf.Checks.Shuffled, s)
	status.StartStatServer(s, &conf.Stats)

	loop.Run()
	return nil
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
		&cli.BoolFlag{
			Name:    ConfigDump,
			Aliases: []string{"D"},
			Value:   false,
			Usage:   "dump parsed configuration and quit",
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
