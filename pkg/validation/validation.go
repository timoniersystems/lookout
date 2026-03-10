// Package validation provides input validation utilities for security and data integrity.
package validation

import (
	"bufio"
	"encoding/json"
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

// ValidateCVETextFile validates that a file contains CVE IDs (one per line, optionally with PURL).
// Empty lines and lines starting with # are skipped.
func ValidateCVETextFile(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	validLines := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Lines can be "CVE-YYYY-NNNNN" or "CVE-YYYY-NNNNN,purl"
		parts := strings.SplitN(line, ",", 2)
		cveID := strings.TrimSpace(parts[0])

		if !strings.HasPrefix(strings.ToUpper(cveID), "CVE-") {
			return fmt.Errorf("invalid CVE file: line %d does not start with 'CVE-' (got: '%s')", lineNum, truncate(line, 80))
		}
		validLines++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	if validLines == 0 {
		return fmt.Errorf("invalid CVE file: no CVE entries found")
	}

	return nil
}

// ValidateTrivyJSON validates that a file is a Trivy JSON scan result.
func ValidateTrivyJSON(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid file: not valid JSON (%v)", err)
	}

	if _, ok := parsed["SchemaVersion"]; !ok {
		return fmt.Errorf("invalid file: does not appear to be a Trivy JSON scan (missing 'SchemaVersion' field)")
	}

	if _, ok := parsed["Results"]; !ok {
		return fmt.Errorf("invalid file: does not appear to be a Trivy JSON scan (missing 'Results' field)")
	}

	return nil
}

// ValidateCycloneDXBOM validates that a file is a CycloneDX BOM.
func ValidateCycloneDXBOM(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid SBOM: not valid JSON (%v)", err)
	}

	bomFormat, ok := parsed["bomFormat"]
	if !ok {
		return fmt.Errorf("invalid SBOM: does not appear to be a CycloneDX BOM (missing 'bomFormat' field)")
	}

	if fmt.Sprintf("%v", bomFormat) != "CycloneDX" {
		return fmt.Errorf("invalid SBOM: bomFormat is '%v', expected 'CycloneDX'", bomFormat)
	}

	components, ok := parsed["components"]
	if !ok {
		return fmt.Errorf("invalid SBOM: does not appear to be a CycloneDX BOM (missing 'components' field)")
	}

	compArray, ok := components.([]interface{})
	if !ok || len(compArray) == 0 {
		return fmt.Errorf("invalid SBOM: 'components' field is empty or not an array")
	}

	return nil
}

// ValidateSPDXBOM validates that a file is an SPDX BOM (JSON format).
func ValidateSPDXBOM(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid SBOM: not valid JSON (%v)", err)
	}

	spdxVersion, ok := parsed["spdxVersion"]
	if !ok {
		return fmt.Errorf("invalid SBOM: does not appear to be an SPDX BOM (missing 'spdxVersion' field)")
	}

	versionStr := fmt.Sprintf("%v", spdxVersion)
	if !strings.HasPrefix(versionStr, "SPDX-") {
		return fmt.Errorf("invalid SBOM: spdxVersion is '%v', expected to start with 'SPDX-'", spdxVersion)
	}

	packages, ok := parsed["packages"]
	if !ok {
		return fmt.Errorf("invalid SBOM: does not appear to be an SPDX BOM (missing 'packages' field)")
	}

	pkgArray, ok := packages.([]interface{})
	if !ok || len(pkgArray) == 0 {
		return fmt.Errorf("invalid SBOM: 'packages' field is empty or not an array")
	}

	return nil
}

// DetectBOMFormat detects whether a JSON file is CycloneDX or SPDX.
// Returns "cyclonedx", "spdx", or empty string if neither.
func DetectBOMFormat(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("invalid SBOM: not valid JSON (%v)", err)
	}

	if bomFormat, ok := parsed["bomFormat"]; ok {
		if fmt.Sprintf("%v", bomFormat) == "CycloneDX" {
			return "cyclonedx", nil
		}
	}

	if spdxVersion, ok := parsed["spdxVersion"]; ok {
		if strings.HasPrefix(fmt.Sprintf("%v", spdxVersion), "SPDX-") {
			return "spdx", nil
		}
	}

	return "", fmt.Errorf("invalid SBOM: unrecognized format (expected CycloneDX or SPDX)")
}

// MaxBase64Length is the maximum length of a continuous base64-like string allowed in uploads.
// Legitimate fields (checksums, hashes, short tokens) are well under this limit.
const MaxBase64Length = 256

// Patterns for detecting obfuscated or suspicious content
var (
	// Long continuous base64 strings (A-Za-z0-9+/=, at least MaxBase64Length chars)
	base64Pattern = regexp.MustCompile(`[A-Za-z0-9+/=]{` + fmt.Sprintf("%d", MaxBase64Length) + `,}`)

	// Data URIs embedding encoded content
	dataURIPattern = regexp.MustCompile(`(?i)data:[a-z0-9/+.-]+;base64,`)

	// Hex escape sequences (e.g., \x41\x42 or 0x41 0x42)
	hexEscapePattern = regexp.MustCompile(`(\\x[0-9a-fA-F]{2}){4,}`)

	// Unicode escape sequences (e.g., \u0041\u0042)
	unicodeEscapePattern = regexp.MustCompile(`(\\u[0-9a-fA-F]{4}){4,}`)

	// Embedded script/code patterns
	scriptPatterns = regexp.MustCompile(`(?i)(<script|javascript:|eval\s*\(|exec\s*\(|system\s*\(|passthru\s*\(|shell_exec\s*\()`)

	// Shell injection patterns - only match at line start (not inside JSON string values
	// like Docker layer history which legitimately contains "/bin/sh -c")
	shellPatterns = regexp.MustCompile(`(?im)(^#!/bin/|^\s*bash\s+-c\s+|^\s*sh\s+-c\s+|^\s*.*\|\s*base64\s+-d|;\s*rm\s+-rf\s+/|&&\s*curl\s|&&\s*wget\s)`)
)

// DetectObfuscatedContent scans a file for obfuscated or suspicious content
// that doesn't belong in CVE lists, Trivy scans, or SBOM files.
func DetectObfuscatedContent(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	if base64Pattern.MatchString(content) {
		return fmt.Errorf("rejected: file contains suspicious base64-encoded content (string longer than %d characters)", MaxBase64Length)
	}

	if dataURIPattern.MatchString(content) {
		return fmt.Errorf("rejected: file contains embedded data URI with base64 content")
	}

	if hexEscapePattern.MatchString(content) {
		return fmt.Errorf("rejected: file contains hex-encoded escape sequences")
	}

	if unicodeEscapePattern.MatchString(content) {
		return fmt.Errorf("rejected: file contains unicode escape sequences")
	}

	if scriptPatterns.MatchString(content) {
		return fmt.Errorf("rejected: file contains embedded script or code execution patterns")
	}

	if shellPatterns.MatchString(content) {
		return fmt.Errorf("rejected: file contains shell command patterns")
	}

	return nil
}

// truncate shortens a string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SanitizeCVEID sanitizes a CVE ID by trimming whitespace and converting to uppercase
func SanitizeCVEID(cveID string) string {
	return strings.TrimSpace(strings.ToUpper(cveID))
}

// SanitizePURL sanitizes a PURL by trimming whitespace
func SanitizePURL(purl string) string {
	return strings.TrimSpace(purl)
}
