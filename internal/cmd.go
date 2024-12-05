/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package internal

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
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

func Run(cCtx *cli.Context) error {
	LogSetup(cCtx.Bool(ConfigDebug))
	dump := cCtx.Bool(ConfigDump)
	conf := ReadConf(cCtx.Path(ConfigConfig), dump)

	if dump {
		return nil
	}

	checks := conf.GetChecks()
	delays := conf.GetDelays()
	da := conf.GetDownAction()

	loop := &Loop{
		Checks:     checks,
		Delays:     delays,
		DownAction: da,
		Shuffle:    conf.Checks.Shuffled,
	}

	loop.Run()
	return nil
}

func Cmd(version string) {
	flags := []cli.Flag{
		&cli.PathFlag{
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

	app := &cli.App{
		Name:    AppName,
		Usage:   AppShort,
		Version: version,
		Flags:   flags,
		Action:  Run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}