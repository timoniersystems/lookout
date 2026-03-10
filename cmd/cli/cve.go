package main

import (
	"github.com/spf13/cobra"
	"github.com/timoniersystems/lookout/pkg/cli/cli_processor"
)

var cveCmd = &cobra.Command{
	Use:   "cve <cve-id>",
	Short: "Fetch and display CVE data from NVD",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cli_processor.ProcessCVEData([]string{args[0]}, severity)
	},
}

func init() {
	rootCmd.AddCommand(cveCmd)
}
