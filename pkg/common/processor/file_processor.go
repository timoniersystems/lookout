// Package processor provides utilities for reading and processing CVE data
// from various input formats including Trivy JSON and plain text files.
package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProcessFileInput processes a file containing CVE data and returns a map of CVE IDs to PURLs.
// Supports both JSON (Trivy format) and plain text files.
// Returns an error if the file doesn't exist, is a directory, or has an unsupported format.
func ProcessFileInput(filePath string) (map[string]string, error) {
	fileInfo, err := os.Stat(filePath)

	if err != nil {
		if os.IsNotExist(err) {

			return nil, fmt.Errorf("file does not exist: %s", filePath)
		}

		return nil, fmt.Errorf("error accessing file: %v", err)
	}
	if fileInfo.IsDir() {

		return nil, fmt.Errorf("the path is a directory, not a file: %s", filePath)
	}

	fileType := filepath.Ext(filePath)

	switch strings.ToLower(fileType) {
	case ".json":
		cvePurlMap, err := ParseTrivyJSONFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CVE IDs from JSON file: %v", err)
		}

		return cvePurlMap, nil

	case ".txt":
		cvePurlMap, err := ReadCVEIDsFromTextFile(filePath)

		if err != nil {
			return nil, fmt.Errorf("failed to read CVE IDs from text file: %v", err)
		}

		return cvePurlMap, nil

	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}
}

// ProcessFileInputForCVEs processes a file and returns only the CVE IDs as a slice.
// This is a convenience function that extracts just the CVE IDs without the PURL associations.
// Supports both JSON (Trivy format) and plain text files.
func ProcessFileInputForCVEs(filePath string) ([]string, error) {
	fileType := filepath.Ext(filePath)

	switch strings.ToLower(fileType) {
	case ".json":
		cvePurlMap, err := ParseTrivyJSONFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CVE IDs from JSON file: %v", err)
		}
		var cveIDs []string

		for cveID := range cvePurlMap {
			cveIDs = append(cveIDs, cveID)
		}
		return cveIDs, nil

	case ".txt":
		cvePurlMap, err := ReadCVEIDsFromTextFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CVE IDs from text file: %v", err)
		}
		var cveIDs []string

		for cveID := range cvePurlMap {
			cveIDs = append(cveIDs, cveID)
		}
		return cveIDs, nil

	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}
}
