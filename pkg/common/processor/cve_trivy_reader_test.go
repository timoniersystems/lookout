package processor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTrivyJSONFile_ValidFile(t *testing.T) {
	content := `{
  "Results": [
    {
      "Vulnerabilities": [
        {
          "VulnerabilityID": "CVE-2021-36159",
          "PkgIdentifier": {
            "PURL": "pkg:npm/qs@4.0.0"
          },
          "InstalledVersion": "4.0.0",
          "FixedVersion": "6.5.3"
        },
        {
          "VulnerabilityID": "CVE-2022-12345",
          "PkgIdentifier": {
            "PURL": "pkg:maven/commons-io@1.0.0"
          },
          "InstalledVersion": "1.0.0",
          "FixedVersion": "2.7.0"
        }
      ]
    }
  ]
}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "trivy_results.json")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ParseTrivyJSONFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := map[string]string{
		"CVE-2021-36159": "pkg:npm/qs@4.0.0",
		"CVE-2022-12345": "pkg:maven/commons-io@1.0.0",
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

func TestParseTrivyJSONFile_MultipleResults(t *testing.T) {
	content := `{
  "Results": [
    {
      "Vulnerabilities": [
        {
          "VulnerabilityID": "CVE-2021-11111",
          "PkgIdentifier": {
            "PURL": "pkg:npm/package1@1.0.0"
          }
        }
      ]
    },
    {
      "Vulnerabilities": [
        {
          "VulnerabilityID": "CVE-2021-22222",
          "PkgIdentifier": {
            "PURL": "pkg:npm/package2@2.0.0"
          }
        }
      ]
    }
  ]
}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "trivy_multi.json")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ParseTrivyJSONFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 CVEs, got %d", len(result))
	}

	if result["CVE-2021-11111"] != "pkg:npm/package1@1.0.0" {
		t.Errorf("Unexpected PURL for CVE-2021-11111: %s", result["CVE-2021-11111"])
	}

	if result["CVE-2021-22222"] != "pkg:npm/package2@2.0.0" {
		t.Errorf("Unexpected PURL for CVE-2021-22222: %s", result["CVE-2021-22222"])
	}
}

func TestParseTrivyJSONFile_NonCVEVulnerabilities(t *testing.T) {
	content := `{
  "Results": [
    {
      "Vulnerabilities": [
        {
          "VulnerabilityID": "CVE-2021-36159",
          "PkgIdentifier": {
            "PURL": "pkg:npm/qs@4.0.0"
          }
        },
        {
          "VulnerabilityID": "GHSA-1234-5678-9abc",
          "PkgIdentifier": {
            "PURL": "pkg:npm/other@1.0.0"
          }
        },
        {
          "VulnerabilityID": "DLA-9999",
          "PkgIdentifier": {
            "PURL": "pkg:deb/debian/package@1.0"
          }
        }
      ]
    }
  ]
}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "trivy_mixed.json")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ParseTrivyJSONFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected only CVE entries (1), got %d", len(result))
	}

	if _, exists := result["CVE-2021-36159"]; !exists {
		t.Error("Expected CVE-2021-36159 to be present")
	}

	if _, exists := result["GHSA-1234-5678-9abc"]; exists {
		t.Error("Expected GHSA entry to be filtered out")
	}

	if _, exists := result["DLA-9999"]; exists {
		t.Error("Expected DLA entry to be filtered out")
	}
}

func TestParseTrivyJSONFile_EmptyResults(t *testing.T) {
	content := `{
  "Results": []
}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "trivy_empty.json")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ParseTrivyJSONFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty map for empty results, got %d entries", len(result))
	}
}

func TestParseTrivyJSONFile_NoVulnerabilities(t *testing.T) {
	content := `{
  "Results": [
    {
      "Vulnerabilities": []
    }
  ]
}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "trivy_no_vulns.json")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ParseTrivyJSONFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty map for no vulnerabilities, got %d entries", len(result))
	}
}

func TestParseTrivyJSONFile_InvalidJSON(t *testing.T) {
	content := `{this is not valid json`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ParseTrivyJSONFile(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestParseTrivyJSONFile_FileNotFound(t *testing.T) {
	_, err := ParseTrivyJSONFile("/nonexistent/path/trivy.json")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestParseTrivyJSONFile_CaseInsensitiveCVE(t *testing.T) {
	content := `{
  "Results": [
    {
      "Vulnerabilities": [
        {
          "VulnerabilityID": "cve-2021-36159",
          "PkgIdentifier": {
            "PURL": "pkg:npm/qs@4.0.0"
          }
        },
        {
          "VulnerabilityID": "CVE-2022-12345",
          "PkgIdentifier": {
            "PURL": "pkg:maven/commons-io@1.0.0"
          }
        }
      ]
    }
  ]
}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "trivy_case.json")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ParseTrivyJSONFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 CVEs (lowercase should be accepted), got %d", len(result))
	}

	if _, exists := result["cve-2021-36159"]; !exists {
		t.Error("Expected lowercase CVE to be accepted")
	}
}
