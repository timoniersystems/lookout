package cli_processor

import (
	"flag"
	"fmt"
	"os"
)

const Version = "1.0"

type CLIArgs struct {
	CVEID            string
	CVEFilePath      string // Renamed from FilePath for clarity
	SBOMPath         string // Renamed from TrivyFilePath for clarity
	OutputPath       string // Renamed from TrivyResultsFile for clarity
	DepPathPURL      string // PURL to trace dependency path for (renamed from TraversePURL)
	Severity         string // Severity filter: "all", "critical", "high", "medium", "low" (default: "high")
	Debug            bool   // Enable debug logging
}

// printHelp displays comprehensive usage information for the CLI
func printHelp() {
	fmt.Println("Lookout - CycloneDX SBOM and CVE Vulnerability Analysis Tool")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  lookout [OPTIONS]")
	fmt.Println()
	fmt.Println("CVE OPERATIONS:")
	fmt.Println("  -cve <CVE-ID>              Fetch and display CVE data from NVD")
	fmt.Println("  -cve-file <file>           Process file containing CVE IDs or SARIF data")
	fmt.Println()
	fmt.Println("SBOM OPERATIONS:")
	fmt.Println("  -sbom <file>                  Run Trivy vulnerability scan on SBOM")
	fmt.Println("  -sbom <file> -output <file>   Save Trivy scan results to file")
	fmt.Println("  -sbom <file> -dep-path <purl> Show transitive dependency path to root")
	fmt.Println()
	fmt.Println("  Note: Dependency path tracing requires Dgraph database.")
	fmt.Println("        Start with: docker-compose --profile standalone up dgraph")
	fmt.Println()
	fmt.Println("OUTPUT FILTERING:")
	fmt.Println("  -severity <level>          Filter CVEs by severity (default: high)")
	fmt.Println("                             Options: all, critical, high, medium, low")
	fmt.Println("                             - critical: CRITICAL only")
	fmt.Println("                             - high:     HIGH and CRITICAL (default)")
	fmt.Println("                             - medium:   MEDIUM and above")
	fmt.Println("                             - low:      LOW and above (all)")
	fmt.Println("                             - all:      Show all CVEs")
	fmt.Println()
	fmt.Println("OTHER OPTIONS:")
	fmt.Println("  -debug                     Enable debug logging (shows detailed operation info)")
	fmt.Println("  -h, -help                  Show this help message")
	fmt.Println("  -version                   Show version information")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Fetch single CVE data")
	fmt.Println("  lookout -cve CVE-2021-44228")
	fmt.Println()
	fmt.Println("  # Fetch CVE data with severity filter")
	fmt.Println("  lookout -cve CVE-2021-44228 -severity all")
	fmt.Println()
	fmt.Println("  # Process file of CVE IDs")
	fmt.Println("  lookout -cve-file cve-list.txt")
	fmt.Println()
	fmt.Println("  # Scan SBOM with Trivy")
	fmt.Println("  lookout -sbom sbom.json")
	fmt.Println()
	fmt.Println("  # Scan SBOM and save results")
	fmt.Println("  lookout -sbom sbom.json -output results.json")
	fmt.Println()
	fmt.Println("  # Show transitive dependency path (identifies which package to upgrade)")
	fmt.Println("  lookout -sbom sbom.json -dep-path 'pkg:npm/express@4.17.1'")
	fmt.Println()
	fmt.Println("For more information, visit: https://github.com/timonier/lookout")
}

func ParseCLIArgs(args []string) CLIArgs {
	flags := flag.NewFlagSet("CLI", flag.ContinueOnError)
	flags.Usage = printHelp // Set custom usage function

	var cveID string
	var cveFilePath string
	var sbomPath string
	var outputPath string
	var depPathPURL string
	var severity string
	var debug bool
	var showHelp bool
	var showVersion bool

	// Define CLI flags
	flags.StringVar(&cveID, "cve", "", "CVE ID to fetch data for")
	flags.StringVar(&cveFilePath, "cve-file", "", "Path to file containing CVE IDs or SARIF data")
	flags.StringVar(&sbomPath, "sbom", "", "Path to SBOM file for Trivy scanning or dependency analysis")
	flags.StringVar(&outputPath, "output", "", "Output file for Trivy scan results (optional)")
	flags.StringVar(&depPathPURL, "dep-path", "", "PURL to trace transitive dependency path for (requires -sbom)")
	flags.StringVar(&severity, "severity", "high", "Minimum severity level to display (all, critical, high, medium, low)")
	flags.BoolVar(&debug, "debug", false, "Enable debug logging")
	flags.BoolVar(&showHelp, "h", false, "Show help message")
	flags.BoolVar(&showHelp, "help", false, "Show help message")
	flags.BoolVar(&showVersion, "version", false, "Show version information")

	flags.Parse(args)

	// Handle help flag
	if showHelp {
		printHelp()
		os.Exit(0)
	}

	// Handle version flag
	if showVersion {
		fmt.Printf("lookout version %s\n", Version)
		os.Exit(0)
	}

	// Validate dependency path arguments
	if depPathPURL != "" && sbomPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -dep-path requires -sbom to be specified.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: lookout -sbom <sbom-file> -dep-path <purl>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintln(os.Stderr, "  lookout -sbom sbom.json -dep-path 'pkg:npm/express@4.17.1'")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Run 'lookout -help' for more information.")
		os.Exit(1)
	}

	return CLIArgs{
		CVEID:       cveID,
		CVEFilePath: cveFilePath,
		SBOMPath:    sbomPath,
		OutputPath:  outputPath,
		DepPathPURL: depPathPURL,
		Severity:    severity,
		Debug:       debug,
	}
}
