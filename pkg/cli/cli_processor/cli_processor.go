package cli_processor

import (
	"flag"
	"fmt"
	"os"
)

type CLIArgs struct {
	CVEID            string
	FilePath         string
	TrivyFilePath    string
	TrivyResultsFile string
	PURLTraversal    bool
	PURL             string
	File             string
}

func ParseCLIArgs(args []string) CLIArgs {
	flags := flag.NewFlagSet("CLI", flag.ContinueOnError)
	var cveID string
	var filePath string
	var trivyFilePath string
	var trivyResultsFile string
	var pURLTraversal bool
	var purl string
	var file string

	flags.StringVar(&cveID, "cve", "", "CVE ID to fetch data for")
	flags.StringVar(&filePath, "f", "", "Path to file containing CVE IDs or SARIF data")
	flags.StringVar(&trivyFilePath, "sbom", "", "Path to SBOM File meant to be consumed by Trivy for processing.")
	flags.StringVar(&trivyResultsFile, "o", "", "Fileame for Trivy to store scan results of (Optional Argument).")
	flags.BoolVar(&pURLTraversal, "traversal", false, "Enable traversal of dependency pathing with a required PURL and CycloneDX SBOM File.")
	flags.StringVar(&purl, "purl", "", "PURL value for traversal")
	flags.StringVar(&file, "file", "", "Path to CycloneDX SBOM JSON file")
	flags.Parse(args)

	if pURLTraversal && (file == "" || purl == "") {
		fmt.Println("Error: `-traversal` requires both `-file` and `-purl` arguments.")
		fmt.Println("Usage: -traversal -file <filePath> -purl <purl>")
		os.Exit(1)
	}

	return CLIArgs{
		CVEID:            cveID,
		FilePath:         filePath,
		TrivyFilePath:    trivyFilePath,
		TrivyResultsFile: trivyResultsFile,
		PURLTraversal:    pURLTraversal,
		PURL:             purl,
		File:             file,
	}
}
