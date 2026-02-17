// Package trivy provides utilities for running Trivy vulnerability scans.
package trivy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RunTrivy runs a Trivy SBOM scan on the specified file.
// If trivyResultsFilename is provided, results are saved to outputs/<filename>.
// Otherwise, results are saved to a temporary file.
// Returns the path to the results file.
func RunTrivy(sbomFilePath string, trivyResultsFilename ...string) (string, error) {
	// Resolve output path
	outputFile, outputDirectory, err := resolveOutputPath(trivyResultsFilename...)
	if err != nil {
		return "", err
	}

	// Ensure output directory exists
	if err := ensureOutputDirectory(outputDirectory); err != nil {
		return "", err
	}

	// Run Trivy command
	if err := executeTrivyScan(sbomFilePath, outputFile); err != nil {
		return "", err
	}

	// Log output location if user-specified filename
	if len(trivyResultsFilename) > 0 {
		fmt.Printf("Trivy SBOM scan output saved to: %s\n", outputFile)
	}

	return outputFile, nil
}

// resolveOutputPath determines the output file and directory paths.
func resolveOutputPath(trivyResultsFilename ...string) (outputFile, outputDirectory string, err error) {
	if len(trivyResultsFilename) > 0 {
		// User-specified output filename
		rootDirectory, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("error getting current working directory: %w", err)
		}

		outputDirectory = filepath.Join(rootDirectory, "outputs")
		outputFile = filepath.Join(outputDirectory, trivyResultsFilename[0])
	} else {
		// Temporary output file
		outputDirectory, err = os.MkdirTemp("", "trivy-results")
		if err != nil {
			return "", "", fmt.Errorf("error creating temporary directory: %w", err)
		}

		tempFile, err := os.CreateTemp(outputDirectory, "*.json")
		if err != nil {
			return "", "", fmt.Errorf("error creating temporary file: %w", err)
		}
		defer func() { _ = tempFile.Close() }()

		outputFile = tempFile.Name()
	}

	return outputFile, outputDirectory, nil
}

// ensureOutputDirectory creates the output directory if it doesn't exist.
func ensureOutputDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}
	return nil
}

// executeTrivyScan runs the Trivy command and captures any errors.
func executeTrivyScan(sbomFilePath, outputFile string) error {
	cmd := exec.Command("trivy", "sbom", "--format", "json", "-o", outputFile, sbomFilePath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running Trivy scan: %s, %w", stderr.String(), err)
	}

	return nil
}
