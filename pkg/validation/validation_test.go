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

func TestValidateCVETextFile(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(name, content string) string {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		return path
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			"Valid CVE-only lines",
			writeFile("valid.txt", "CVE-2021-36159\nCVE-2021-30139\n"),
			false, "",
		},
		{
			"Valid CVE with PURL",
			writeFile("valid_purl.txt", "CVE-2021-36159, pkg:apk/alpine/apk-tools@2.10.4\nCVE-2021-30139\n"),
			false, "",
		},
		{
			"Valid with comments and blank lines",
			writeFile("comments.txt", "# This is a comment\n\nCVE-2021-36159\n\n# Another comment\nCVE-2021-30139\n"),
			false, "",
		},
		{
			"Invalid line",
			writeFile("invalid.txt", "CVE-2021-36159\nsome random text\nCVE-2021-30139\n"),
			true, "line 2",
		},
		{
			"Empty file",
			writeFile("empty.txt", ""),
			true, "no CVE entries",
		},
		{
			"Only comments and blanks",
			writeFile("comments_only.txt", "# comment\n\n# another\n"),
			true, "no CVE entries",
		},
		{
			"Non-existent file",
			filepath.Join(tmpDir, "nonexistent.txt"),
			true, "failed to open",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCVETextFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCVETextFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateCVETextFile() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateTrivyJSON(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(name, content string) string {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		return path
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			"Valid Trivy JSON",
			writeFile("valid.json", `{"SchemaVersion": 2, "Results": []}`),
			false, "",
		},
		{
			"Missing SchemaVersion",
			writeFile("no_schema.json", `{"Results": []}`),
			true, "SchemaVersion",
		},
		{
			"Missing Results",
			writeFile("no_results.json", `{"SchemaVersion": 2}`),
			true, "Results",
		},
		{
			"Invalid JSON",
			writeFile("invalid.json", `not json at all`),
			true, "not valid JSON",
		},
		{
			"Random JSON object",
			writeFile("random.json", `{"foo": "bar", "baz": 42}`),
			true, "SchemaVersion",
		},
		{
			"Non-existent file",
			filepath.Join(tmpDir, "nonexistent.json"),
			true, "failed to read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTrivyJSON(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTrivyJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateTrivyJSON() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateCycloneDXBOM(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(name, content string) string {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		return path
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			"Valid CycloneDX BOM",
			writeFile("valid.json", `{"bomFormat": "CycloneDX", "specVersion": "1.4", "components": [{"name": "test"}]}`),
			false, "",
		},
		{
			"Missing bomFormat",
			writeFile("no_format.json", `{"specVersion": "1.4", "components": [{"name": "test"}]}`),
			true, "bomFormat",
		},
		{
			"Wrong bomFormat",
			writeFile("wrong_format.json", `{"bomFormat": "SPDX", "components": [{"name": "test"}]}`),
			true, "expected 'CycloneDX'",
		},
		{
			"Missing components",
			writeFile("no_components.json", `{"bomFormat": "CycloneDX", "specVersion": "1.4"}`),
			true, "components",
		},
		{
			"Empty components",
			writeFile("empty_components.json", `{"bomFormat": "CycloneDX", "specVersion": "1.4", "components": []}`),
			true, "empty",
		},
		{
			"Invalid JSON",
			writeFile("invalid.json", `not json`),
			true, "not valid JSON",
		},
		{
			"Non-existent file",
			filepath.Join(tmpDir, "nonexistent.json"),
			true, "failed to read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCycloneDXBOM(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCycloneDXBOM() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateCycloneDXBOM() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
