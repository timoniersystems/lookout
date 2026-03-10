package processor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessFileInput_JSONFile(t *testing.T) {
	// Create a temporary JSON file with Trivy format
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test.json")

	jsonContent := `{
		"Results": [
			{
				"Vulnerabilities": [
					{
						"VulnerabilityID": "CVE-2021-36159",
						"PkgName": "libfetch",
						"InstalledVersion": "2.34-r0",
						"PkgIdentifier": {
							"PURL": "pkg:apk/alpine/libfetch@2.34-r0?arch=x86_64"
						}
					}
				]
			}
		]
	}`

	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	result, err := ProcessFileInput(jsonFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 CVE, got %d", len(result))
	}

	purl, exists := result["CVE-2021-36159"]
	if !exists {
		t.Error("Expected CVE-2021-36159 in results")
	}

	expectedPURL := "pkg:apk/alpine/libfetch@2.34-r0?arch=x86_64"
	if purl != expectedPURL {
		t.Errorf("Expected PURL %q, got %q", expectedPURL, purl)
	}
}

func TestProcessFileInput_TextFile(t *testing.T) {
	// Create a temporary text file
	tmpDir := t.TempDir()
	txtFile := filepath.Join(tmpDir, "test.txt")

	txtContent := "CVE-2021-36159\nCVE-2022-12345\n"
	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		t.Fatalf("Failed to create test text file: %v", err)
	}

	result, err := ProcessFileInput(txtFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 CVEs, got %d", len(result))
	}

	if _, exists := result["CVE-2021-36159"]; !exists {
		t.Error("Expected CVE-2021-36159 in results")
	}

	if _, exists := result["CVE-2022-12345"]; !exists {
		t.Error("Expected CVE-2022-12345 in results")
	}
}

func TestProcessFileInput_FileNotExist(t *testing.T) {
	_, err := ProcessFileInput("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	// The error is wrapped, so just check that it contains the expected message
	expectedMsg := "file does not exist"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

func TestProcessFileInput_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := ProcessFileInput(tmpDir)
	if err == nil {
		t.Error("Expected error for directory, got nil")
	}
}

func TestProcessFileInput_UnsupportedFileType(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "test.xml")

	if err := os.WriteFile(invalidFile, []byte("<xml></xml>"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := ProcessFileInput(invalidFile)
	if err == nil {
		t.Error("Expected error for unsupported file type, got nil")
	}
}

func TestProcessFileInputForCVEs_JSONFile(t *testing.T) {
	// Create a temporary JSON file with Trivy format
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test.json")

	jsonContent := `{
		"Results": [
			{
				"Vulnerabilities": [
					{
						"VulnerabilityID": "CVE-2021-36159",
						"PkgName": "libfetch",
						"PkgIdentifier": {
							"PURL": "pkg:apk/alpine/libfetch@2.34-r0"
						}
					},
					{
						"VulnerabilityID": "CVE-2022-12345",
						"PkgName": "curl",
						"PkgIdentifier": {
							"PURL": "pkg:apk/alpine/curl@7.79.0"
						}
					}
				]
			}
		]
	}`

	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	result, err := ProcessFileInputForCVEs(jsonFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 CVEs, got %d", len(result))
	}

	// Check that both CVE IDs are present (order doesn't matter)
	foundCVEs := make(map[string]bool)
	for _, cve := range result {
		foundCVEs[cve] = true
	}

	if !foundCVEs["CVE-2021-36159"] {
		t.Error("Expected CVE-2021-36159 in results")
	}

	if !foundCVEs["CVE-2022-12345"] {
		t.Error("Expected CVE-2022-12345 in results")
	}
}

func TestProcessFileInputForCVEs_TextFile(t *testing.T) {
	tmpDir := t.TempDir()
	txtFile := filepath.Join(tmpDir, "test.txt")

	txtContent := "CVE-2021-36159\nCVE-2022-12345\nCVE-2023-99999\n"
	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		t.Fatalf("Failed to create test text file: %v", err)
	}

	result, err := ProcessFileInputForCVEs(txtFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 CVEs, got %d", len(result))
	}
}

func TestProcessFileInputForCVEs_UnsupportedFileType(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "test.csv")

	if err := os.WriteFile(invalidFile, []byte("CVE,PURL\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := ProcessFileInputForCVEs(invalidFile)
	if err == nil {
		t.Error("Expected error for unsupported file type, got nil")
	}
}
