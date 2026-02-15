package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"lookout/pkg/cli/cli_processor"
	"lookout/pkg/common/nvd"
	"lookout/pkg/common/processor"
	"lookout/pkg/common/trivy"
	"lookout/pkg/ui/dgraph"
	"lookout/pkg/logging"
	"lookout/pkg/repository"
	"lookout/pkg/service"
)

func RunCLI(args []string) {
	cliArgs := cli_processor.ParseCLIArgs(args)

	// Initialize logging based on debug flag
	if cliArgs.Debug {
		logging.SetGlobalLevel(logging.DebugLevel)
	} else {
		logging.SetGlobalLevel(logging.InfoLevel)
	}

	// Handle SBOM with dependency path tracing
	if cliArgs.DepPathPURL != "" {
		err := performPurlTraversal(cliArgs.DepPathPURL, cliArgs.SBOMPath)
		if err != nil {
			log.Printf("Error performing PURL traversal: %v", err)
			os.Exit(1)
		}
		return
	}

	// Handle single CVE lookup
	if cliArgs.CVEID != "" {
		cli_processor.ProcessCVEData([]string{cliArgs.CVEID}, cliArgs.Severity)
		return
	}

	// Handle CVE file processing
	if cliArgs.CVEFilePath != "" {
		cvePurlMap, err := processor.ProcessFileInput(cliArgs.CVEFilePath)
		if err != nil {
			log.Printf("No such file or directory: %v", err)
			return
		}
		pairs, err := nvd.FetchCVEDataWithPURLs(cvePurlMap)
		if err != nil {
			log.Printf("Failed to extract CVE IDs from the uploaded file: %v", err)
			return
		}
		cli_processor.ProcessCVEDataWithPURLs(pairs, cliArgs.Severity)
		return
	}

	// Handle SBOM scanning
	if cliArgs.SBOMPath != "" {
		if !trivy.CheckTrivyInstalled() {
			log.Println("Please install Trivy before running this application.")
			return
		}

		var trivyResults string
		var err error

		if cliArgs.OutputPath != "" {
			trivyResults, err = trivy.RunTrivy(cliArgs.SBOMPath, cliArgs.OutputPath)
		} else {
			trivyResults, err = trivy.RunTrivy(cliArgs.SBOMPath)
		}

		if err != nil {
			log.Printf("Failed to run Trivy: %v", err)
			return
		}

		cvePurlMap, err := processor.ProcessFileInput(trivyResults)
		if err != nil {
			log.Printf("Failed to process file input: %v", err)
			return
		}

		pairs, err := nvd.FetchCVEDataWithPURLs(cvePurlMap)
		if err != nil {
			log.Printf("Failed to fetch CVE data: %v", err)
			return
		}
		cli_processor.ProcessCVEDataWithPURLs(pairs, cliArgs.Severity)
		return
	}

	// No arguments provided
	fmt.Fprintln(os.Stderr, "No operation specified. Run 'lookout -help' for usage information.")
	os.Exit(1)
}

// performPurlTraversal performs PURL traversal using the service layer directly
func performPurlTraversal(pURL, filePath string) error {
	// Validate file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("SBOM file not found: %s", filePath)
	}

	// Get global client manager and create repository
	clientManager := dgraph.GetGlobalClientManager()
	repo := repository.NewDgraphRepository(clientManager)
	vulnerabilityService := service.NewVulnerabilityService(repo)
	defer func() {
		if err := repo.Close(); err != nil {
			log.Printf("Warning: Failed to close repository: %v", err)
		}
	}()

	// Drop existing data first to allow schema changes
	ctx := context.Background()
	if err := repo.DropAllData(ctx); err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}

	// Setup Dgraph schema (required for CLI operations)
	dgraph.SetupAndRunDgraph()

	// Process PURL traversal (will drop data again, but that's okay)
	result, err := vulnerabilityService.ProcessPURLTraversal(ctx, filePath, pURL)
	if err != nil {
		return fmt.Errorf("failed to process PURL traversal: %w", err)
	}

	// Display ASCII art visualization
	printASCIITree(result, pURL)

	// Display JSON output for complete information
	fmt.Println("\n" + strings.Repeat("─", 70))
	fmt.Println("JSON OUTPUT")
	fmt.Println(strings.Repeat("─", 70))
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Warning: Failed to marshal result to JSON: %v", err)
	} else {
		fmt.Println(string(jsonData))
	}
	fmt.Println(strings.Repeat("─", 70))

	return nil
}

// printASCIITree displays a beautiful ASCII art tree of the dependency path
func printASCIITree(result dgraph.FilteredResult, searchedPURL string) {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 70))
	fmt.Println("  DEPENDENCY PATH ANALYSIS")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Println()

	if len(result.PathFromRootPackage) == 0 {
		fmt.Println("  ⚠️  No path to root package found.")
		fmt.Println()
		fmt.Println(strings.Repeat("═", 70))
		return
	}

	fmt.Printf("  Searched: %s\n", searchedPURL)
	fmt.Printf("  Depth:    %d level(s)\n", len(result.PathFromRootPackage)-1)
	fmt.Println()
	fmt.Println("  Dependency Tree:")
	fmt.Println()

	// Draw the tree (root → vulnerable)
	for i := 0; i < len(result.PathFromRootPackage); i++ {
		dep := result.PathFromRootPackage[i]
		isFirst := i == 0 // First is the root package
		isLast := i == len(result.PathFromRootPackage)-1 // Last is the vulnerable package
		depth := i

		// Build the tree structure
		var prefix, line, icon string

		if isFirst {
			// First item (root package)
			icon = "🏠"
			prefix = "     "
			line = fmt.Sprintf("%s %s", icon, dep)
		} else if isLast {
			// Last item (vulnerable component)
			icon = "⚠️"
			prefix = "     "
			for j := 0; j < depth-1; j++ {
				prefix += "│    "
			}
			line = fmt.Sprintf("%s└──> %s %s", prefix, icon, dep)
		} else {
			// Middle items (dependencies)
			icon = "📦"
			prefix = "     "
			for j := 0; j < depth-1; j++ {
				prefix += "│    "
			}
			line = fmt.Sprintf("%s└──> %s %s", prefix, icon, dep)
		}

		fmt.Println(line)

		// Add connector for next item
		if !isLast {
			connector := "     "
			for j := 0; j < depth; j++ {
				connector += "│    "
			}
			fmt.Println(connector)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("═", 70))
	fmt.Println()
	fmt.Println("  Legend:")
	fmt.Println("    🏠  = Root package (your application)")
	fmt.Println("    📦  = Intermediate dependency")
	fmt.Println("    ⚠️  = Vulnerable component")
	fmt.Println()
	fmt.Println(strings.Repeat("═", 70))
}

func main() {
	RunCLI(os.Args[1:])
}
