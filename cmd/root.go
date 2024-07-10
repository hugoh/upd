/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package cmd

import (
	"os"

	"github.com/hugoh/upd/internal"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   internal.AppName,
	Short: internal.AppShort,
	Long:  internal.AppDesc,
	Run: func(cmd *cobra.Command, args []string) {
		err := internal.ReadConf(cfgFile)
		internal.FatalIfError(err)
		internal.LogSetup(debug)
		checks, errC := internal.GetChecksFromConf()
		internal.FatalIfError(errC)
		delays, errD := internal.GetDelaysFromConf()
		internal.FatalIfError(errD)
		da, errA := internal.GetDownActionFromConf()
		internal.FatalIfError(errA)

		loop := &internal.Loop{
			Checks:     checks,
			Delays:     delays,
			DownAction: da,
		}

		if dumpConf {
			internal.DumpConf(loop)
			return
		}

		loop.Run()
	},
}

var cfgFile string
var debug bool
var dumpConf bool

// Execute adds all child commands to the root command and sets flags
// appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().
		StringVar(&cfgFile, "config", "", "config file (default is $HOME/.up.yaml)")
	rootCmd.PersistentFlags().
		BoolVarP(&debug, "debug", "d", false, "display debugging output in the console")
	rootCmd.PersistentFlags().
		BoolVarP(&dumpConf, "dump", "D", false, "dump parsed configuration and quit")
}
