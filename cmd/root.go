/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package cmd

import (
	"os"

	"github.com/hugoh/upd/internal"
	"github.com/spf13/cobra"
)

var (
	cfgFile  string //nolint:gochecknoglobals
	debug    bool   //nolint:gochecknoglobals
	dumpConf bool   //nolint:gochecknoglobals
)

func run(_ *cobra.Command, _ []string) {
	internal.LogSetup(debug)
	conf := internal.ReadConf(cfgFile)
	checks := conf.GetChecks()
	delays := conf.GetDelays()
	da := conf.GetDownAction()

	loop := &internal.Loop{
		Checks:     checks,
		Delays:     delays,
		DownAction: da,
		Shuffle:    conf.Checks.Shuffled,
	}

	if dumpConf {
		conf.Dump()
		return
	}

	loop.Run()
}

// Execute adds all child commands to the root command and sets flags
// appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	rootCmd := &cobra.Command{ //nolint:exhaustruct
		Use:   internal.AppName,
		Short: internal.AppShort,
		Long:  internal.AppDesc,
		Run:   run,
	}

	rootCmd.Version = version

	rootCmd.PersistentFlags().
		StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.up.yaml)")
	rootCmd.PersistentFlags().
		BoolVarP(&debug, "debug", "d", false, "display debugging output in the console")
	rootCmd.PersistentFlags().
		BoolVarP(&dumpConf, "dump", "D", false, "dump parsed configuration and quit")

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
