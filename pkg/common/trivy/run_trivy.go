package trivy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func RunTrivy(sbomFilePath string, trivyResultsFilename ...string) (string, error) {
	var outputFile string
	var outputDirectory string

	if len(trivyResultsFilename) > 0 {
		rootDirectory, err := os.Getwd()
		if err != nil {
			fmt.Println("Error getting current working directory:", err)
			return "", fmt.Errorf("Error getting current working directory: %w", err)
		}
		fmt.Println(trivyResultsFilename)
		outputDirectory = filepath.Join(rootDirectory, "outputs")
		outputFile = filepath.Join(outputDirectory, trivyResultsFilename[0])

	} else {
		var err error
		outputDirectory, err = os.MkdirTemp("", "trivy-results")
		if err != nil {
			fmt.Println("Error creating temporary directory:", err)
			return "", fmt.Errorf("Error creating temporary directory: %w", err)
		}
		tempFile, err := os.CreateTemp(outputDirectory, "*.json")
		if err != nil {
			fmt.Println("Error creating temporary file:", err)
			return "", fmt.Errorf("Error creating temporary file: %w", err)
		}
		defer tempFile.Close()
		outputFile = tempFile.Name()
	}

	if err := os.MkdirAll(outputDirectory, 0755); err != nil {

		fmt.Println("Error creating output directory:", err)
		return "", fmt.Errorf("Error creating output directory: %w", err)
	}

	cmd := exec.Command("trivy", "sbom", "--format", "json", "-o", outputFile, sbomFilePath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error running Trivy scan: %s, %w", stderr.String(), err)
	}

	if len(trivyResultsFilename) > 0 {
		fmt.Printf("Trivy SBOM scan output saved to: %s\n", outputFile)
	}

	return outputFile, nil
}
