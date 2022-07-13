package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "Blaze SMTP Listener",
	Short: "SMTP server for Netcore",
	Long:  `SMTP Server for Netcore,Part of Blaze MTA Project`,
	Run:   nil,
}

var (
	verbose bool
)

func init() {
	cobra.OnInitialize()
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"print out more debug information")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if verbose {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logrus.SetLevel(logrus.InfoLevel)
		}
	}
}
