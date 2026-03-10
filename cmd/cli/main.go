package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("lookout version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func main() {
	Execute()
}
