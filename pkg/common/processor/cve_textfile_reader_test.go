package processor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCVEIDsFromTextFile_ValidFile(t *testing.T) {
	content := `CVE-2021-36159,pkg:npm/qs@4.0.0
CVE-2022-12345,pkg:maven/commons-io@1.0.0
CVE-2023-99999`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_cves.txt")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ReadCVEIDsFromTextFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := map[string]string{
		"CVE-2021-36159": "pkg:npm/qs@4.0.0",
		"CVE-2022-12345": "pkg:maven/commons-io@1.0.0",
		"CVE-2023-99999": "",
	}

	if len(result) != len(expected) {
		t.Errorf("Expected %d CVEs, got %d", len(expected), len(result))
	}

	for cve, expectedPurl := range expected {
		if actualPurl, exists := result[cve]; !exists {
			t.Errorf("Expected CVE %s not found in results", cve)
		} else if actualPurl != expectedPurl {
			t.Errorf("For CVE %s: expected PURL %s, got %s", cve, expectedPurl, actualPurl)
		}
	}
}

func TestReadCVEIDsFromTextFile_WithWhitespace(t *testing.T) {
	content := `  CVE-2021-36159  ,  pkg:npm/qs@4.0.0
CVE-2022-12345,pkg:maven/commons-io@1.0.0  `

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_whitespace.txt")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ReadCVEIDsFromTextFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result["CVE-2021-36159"] != "pkg:npm/qs@4.0.0" {
		t.Errorf("Expected whitespace to be trimmed, got: %s", result["CVE-2021-36159"])
	}

	if result["CVE-2022-12345"] != "pkg:maven/commons-io@1.0.0" {
		t.Errorf("Expected whitespace to be trimmed, got: %s", result["CVE-2022-12345"])
	}
}

func TestReadCVEIDsFromTextFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.txt")

	err := os.WriteFile(tmpFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ReadCVEIDsFromTextFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty map for empty file, got %d entries", len(result))
	}
}

func TestReadCVEIDsFromTextFile_FileNotFound(t *testing.T) {
	_, err := ReadCVEIDsFromTextFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestReadCVEIDsFromTextFile_OnlyPURLs(t *testing.T) {
	content := `CVE-2021-11111
CVE-2021-22222
CVE-2021-33333`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_no_purls.txt")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ReadCVEIDsFromTextFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	for cve, purl := range result {
		if purl != "" {
			t.Errorf("Expected empty PURL for CVE %s, got: %s", cve, purl)
		}
	}
}
