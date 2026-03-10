package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/timoniersystems/lookout/pkg/logging"
)

// Version is set by -ldflags at build time.
var Version = "dev"

var (
	severity string
	debug    bool
)

var rootCmd = &cobra.Command{
	Use:   "lookout",
	Short: "CycloneDX SBOM and CVE vulnerability analysis tool",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if debug {
			logging.SetGlobalLevel(logging.DebugLevel)
		} else {
			logging.SetGlobalLevel(logging.InfoLevel)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&severity, "severity", "high", "Minimum severity level to display (all, critical, high, medium, low)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
