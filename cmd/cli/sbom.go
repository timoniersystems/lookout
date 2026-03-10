package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/timoniersystems/lookout/pkg/cli/cli_processor"
	"github.com/timoniersystems/lookout/pkg/common/nvd"
	"github.com/timoniersystems/lookout/pkg/common/processor"
	"github.com/timoniersystems/lookout/pkg/common/trivy"
	"github.com/timoniersystems/lookout/pkg/repository"
	"github.com/timoniersystems/lookout/pkg/service"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
)

var (
	outputPath  string
	depPathPURL string
)

var sbomCmd = &cobra.Command{
	Use:   "sbom <file>",
	Short: "Run Trivy vulnerability scan on an SBOM file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sbomPath := args[0]

		if depPathPURL != "" {
			if err := performPurlTraversal(depPathPURL, sbomPath); err != nil {
				log.Printf("Error performing PURL traversal: %v", err)
				os.Exit(1)
			}
			return
		}

		if !trivy.CheckTrivyInstalled() {
			log.Println("Please install Trivy before running this application.")
			return
		}

		var trivyResults string
		var err error
		if outputPath != "" {
			trivyResults, err = trivy.RunTrivy(sbomPath, outputPath)
		} else {
			trivyResults, err = trivy.RunTrivy(sbomPath)
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
		cli_processor.ProcessCVEDataWithPURLs(pairs, severity)
	},
}

func init() {
	sbomCmd.Flags().StringVar(&outputPath, "output", "", "Output file for Trivy scan results")
	sbomCmd.Flags().StringVar(&depPathPURL, "dep-path", "", "PURL to trace transitive dependency path (requires Dgraph)")
	rootCmd.AddCommand(sbomCmd)
}

func performPurlTraversal(pURL, filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("SBOM file not found: %s", filePath)
	}

	clientManager := dgraph.GetGlobalClientManager()
	repo := repository.NewDgraphRepository(clientManager)
	vulnerabilityService := service.NewVulnerabilityService(repo)
	defer func() {
		if err := repo.Close(); err != nil {
			log.Printf("Warning: Failed to close repository: %v", err)
		}
	}()

	ctx := context.Background()
	if err := repo.DropAllData(ctx); err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}

	dgraph.SetupAndRunDgraph()

	result, err := vulnerabilityService.ProcessPURLTraversal(ctx, filePath, pURL)
	if err != nil {
		return fmt.Errorf("failed to process PURL traversal: %w", err)
	}

	printASCIITree(result, pURL)

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

	for i := 0; i < len(result.PathFromRootPackage); i++ {
		dep := result.PathFromRootPackage[i]
		isFirst := i == 0
		isLast := i == len(result.PathFromRootPackage)-1
		depth := i

		var prefix, line, icon string

		switch {
		case isFirst:
			icon = "🏠"
			prefix = "     "
			line = fmt.Sprintf("%s %s", icon, dep)
		case isLast:
			icon = "⚠️"
			prefix = "     "
			for j := 0; j < depth-1; j++ {
				prefix += "│    "
			}
			line = fmt.Sprintf("%s└──> %s %s", prefix, icon, dep)
		default:
			icon = "📦"
			prefix = "     "
			for j := 0; j < depth-1; j++ {
				prefix += "│    "
			}
			line = fmt.Sprintf("%s└──> %s %s", prefix, icon, dep)
		}

		fmt.Println(line)

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
