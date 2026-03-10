package main

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/timoniersystems/lookout/pkg/cli/cli_processor"
	"github.com/timoniersystems/lookout/pkg/common/nvd"
	"github.com/timoniersystems/lookout/pkg/common/processor"
)

var cveFileCmd = &cobra.Command{
	Use:   "cve-file <file>",
	Short: "Process a file containing CVE IDs or SARIF data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cvePurlMap, err := processor.ProcessFileInput(args[0])
		if err != nil {
			log.Printf("No such file or directory: %v", err)
			return
		}
		pairs, err := nvd.FetchCVEDataWithPURLs(cvePurlMap)
		if err != nil {
			log.Printf("Failed to extract CVE IDs from the uploaded file: %v", err)
			return
		}
		cli_processor.ProcessCVEDataWithPURLs(pairs, severity)
	},
}

func init() {
	rootCmd.AddCommand(cveFileCmd)
}
