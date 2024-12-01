/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package cmd

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/hugoh/upd/internal"
)

type CLI struct {
	Config  string           `help:"config file"                             short:"c" type:"existingfile"`
	Debug   bool             `help:"display debugging output in the console" short:"d"`
	Dump    bool             `help:"dump parsed configuration and quit"      short:"D"`
	Version kong.VersionFlag `help:"print version information"`
}

func (cli *CLI) Run(_ *kong.Context) error {
	internal.LogSetup(cli.Debug)
	conf := internal.ReadConf(cli.Config)
	checks := conf.GetChecks()
	delays := conf.GetDelays()
	da := conf.GetDownAction()

	loop := &internal.Loop{
		Checks:     checks,
		Delays:     delays,
		DownAction: da,
		Shuffle:    conf.Checks.Shuffled,
	}

	if cli.Dump {
		conf.Dump()
		return nil
	}

	loop.Run()
	return nil
}

// Execute adds all child commands to the root command and sets flags
// appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	cli := CLI{}
	ctx := kong.Parse(&cli,
		kong.Name(internal.AppName),
		kong.Description(internal.AppShort),
		kong.UsageOnError(),
		kong.Vars{
			"version": internal.Version(version),
		})

	err := ctx.Run()
	if err != nil {
		os.Exit(1)
	}
}
