package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCVEID(t *testing.T) {
	tests := []struct {
		name    string
		cveID   string
		wantErr bool
	}{
		{"Valid CVE ID", "CVE-2021-36159", false},
		{"Valid CVE ID lowercase", "cve-2021-36159", false},
		{"Valid CVE ID with whitespace", "  CVE-2021-36159  ", false},
		{"Valid CVE ID 5 digits", "CVE-2021-12345", false},
		{"Empty CVE ID", "", true},
		{"Invalid format - no prefix", "2021-36159", true},
		{"Invalid format - wrong prefix", "VUL-2021-36159", true},
		{"Invalid format - 3 digit year", "CVE-999-36159", true},
		{"Invalid format - 3 digit number", "CVE-2021-123", true},
		{"Invalid format - no dash", "CVE202136159", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCVEID(tt.cveID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCVEID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePURL(t *testing.T) {
	tests := []struct {
		name    string
		purl    string
		wantErr bool
	}{
		{"Valid npm PURL", "pkg:npm/qs@4.0.0", false},
		{"Valid maven PURL", "pkg:maven/commons-io/commons-io@1.0.0", false},
		{"Valid composer PURL", "pkg:composer/asm89/stack-cors@1.3.0", false},
		{"Valid PURL with whitespace", "  pkg:npm/qs@4.0.0  ", false},
		{"Empty PURL", "", true},
		{"Invalid format - no pkg prefix", "npm/qs@4.0.0", true},
		{"Invalid format - no type", "pkg:/qs@4.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePURL(tt.purl)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid file path", tmpFile, false},
		{"Empty path", "", true},
		{"Path traversal", "../../../etc/passwd", true},
		{"Non-existent file", "/nonexistent/file.txt", true},
		{"Directory not file", tmpDir, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCVEIDList(t *testing.T) {
	tests := []struct {
		name    string
		cveIDs  []string
		wantErr bool
	}{
		{"Valid list", []string{"CVE-2021-36159", "CVE-2022-12345"}, false},
		{"Empty list", []string{}, true},
		{"List with invalid ID", []string{"CVE-2021-36159", "INVALID"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCVEIDList(tt.cveIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCVEIDList() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeCVEID(t *testing.T) {
	tests := []struct {
		name     string
		cveID    string
		expected string
	}{
		{"Lowercase", "cve-2021-36159", "CVE-2021-36159"},
		{"Whitespace", "  CVE-2021-36159  ", "CVE-2021-36159"},
		{"Mixed case", "CvE-2021-36159", "CVE-2021-36159"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeCVEID(tt.cveID)
			if result != tt.expected {
				t.Errorf("SanitizeCVEID() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizePURL(t *testing.T) {
	tests := []struct {
		name     string
		purl     string
		expected string
	}{
		{"Whitespace", "  pkg:npm/qs@4.0.0  ", "pkg:npm/qs@4.0.0"},
		{"No whitespace", "pkg:npm/qs@4.0.0", "pkg:npm/qs@4.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePURL(tt.purl)
			if result != tt.expected {
				t.Errorf("SanitizePURL() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateFilePathExists(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid file path", tmpFile, false},
		{"Valid file path with whitespace", "  " + tmpFile + "  ", false},
		{"Empty path", "", true},
		{"Non-existent file", "/nonexistent/file.txt", true},
		{"Directory (should pass - less strict)", tmpDir, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePathExists(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePathExists() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
