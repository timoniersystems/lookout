// Package validation provides input validation utilities for security and data integrity.
package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CVE ID format: CVE-YYYY-NNNNN where YYYY is year and NNNNN is 4+ digits
var cveIDPattern = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

// PURL format validation (basic pattern)
// Format: pkg:type/namespace/name@version
var purlPattern = regexp.MustCompile(`^pkg:[a-z\+][a-z0-9\+\.\-]*/.+`)

// ValidateCVEID validates that a CVE ID follows the correct format
func ValidateCVEID(cveID string) error {
	if cveID == "" {
		return fmt.Errorf("CVE ID cannot be empty")
	}

	cveID = strings.TrimSpace(strings.ToUpper(cveID))

	if !cveIDPattern.MatchString(cveID) {
		return fmt.Errorf("invalid CVE ID format: %s (expected format: CVE-YYYY-NNNNN)", cveID)
	}

	return nil
}

// ValidatePURL validates that a Package URL follows the correct format
func ValidatePURL(purl string) error {
	if purl == "" {
		return fmt.Errorf("PURL cannot be empty")
	}

	purl = strings.TrimSpace(purl)

	if !purlPattern.MatchString(purl) {
		return fmt.Errorf("invalid PURL format: %s (expected format: pkg:type/namespace/name@version)", purl)
	}

	return nil
}

// ValidateFilePath validates that a file path is safe and the file exists
func ValidateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	path = strings.TrimSpace(path)

	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected in file path: %s", path)
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check if file exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", cleanPath)
		}
		return fmt.Errorf("error accessing file %s: %w", cleanPath, err)
	}

	// Check if it's a regular file (not a directory)
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", cleanPath)
	}

	return nil
}

// ValidateFilePathExists checks if a file path exists without other security checks
// This is useful for cases where you want to check existence but not enforce security
func ValidateFilePathExists(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	path = strings.TrimSpace(path)

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", path)
		}
		return fmt.Errorf("error accessing file %s: %w", path, err)
	}

	return nil
}

// ValidateCVEIDList validates a list of CVE IDs
func ValidateCVEIDList(cveIDs []string) error {
	if len(cveIDs) == 0 {
		return fmt.Errorf("CVE ID list cannot be empty")
	}

	for i, cveID := range cveIDs {
		if err := ValidateCVEID(cveID); err != nil {
			return fmt.Errorf("invalid CVE ID at index %d: %w", i, err)
		}
	}

	return nil
}

// SanitizeCVEID sanitizes a CVE ID by trimming whitespace and converting to uppercase
func SanitizeCVEID(cveID string) string {
	return strings.TrimSpace(strings.ToUpper(cveID))
}

// SanitizePURL sanitizes a PURL by trimming whitespace
func SanitizePURL(purl string) string {
	return strings.TrimSpace(purl)
}
