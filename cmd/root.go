/*
Copyright © 2024 Hugo Haas <hugoh@hugoh.net>
*/
package cmd

import (
	"log"
	"os"

	"github.com/hugoh/upd/internal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func run(cmd *cobra.Command, _ []string) {
	debug, errD := cmd.Flags().GetBool("debug")
	if errD != nil {
		log.Fatal(errD)
	}
	internal.LogSetup(debug)
	configPath, errC := cmd.Flags().GetString("config")
	if errC != nil {
		log.Fatal(errC)
	}
	conf := internal.ReadConf(configPath)
	checks := conf.GetChecks()
	delays := conf.GetDelays()
	da := conf.GetDownAction()

	loop := &internal.Loop{
		Checks:     checks,
		Delays:     delays,
		DownAction: da,
		Shuffle:    conf.Checks.Shuffled,
	}

	dumpConf, errU := cmd.Flags().GetBool("dump")
	if errU != nil {
		log.Fatal(errU)
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
	v := viper.New()

	rootCmd := &cobra.Command{
		Use:   internal.AppName,
		Short: internal.AppShort,
		Long:  internal.AppDesc,
		Run:   run,
	}

	rootCmd.Version = version

	rootCmd.PersistentFlags().
		StringP("config", "c", "", "config file (default is "+internal.ConfigBase+"."+internal.ConfigType+")")
	rootCmd.PersistentFlags().
		BoolP("debug", "d", false, "display debugging output in the console")
	rootCmd.PersistentFlags().
		BoolP("dump", "D", false, "dump parsed configuration and quit")

	var err error
	err = v.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	if err != nil {
		log.Fatal(err)
	}
	err = v.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	if err != nil {
		log.Fatal(err)
	}
	err = v.BindPFlag("dump", rootCmd.PersistentFlags().Lookup("dump"))
	if err != nil {
		log.Fatal(err)
	}

	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
