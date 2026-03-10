package spdx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBOM_WithPURLs(t *testing.T) {
	content := `{
		"SPDXID": "SPDXRef-DOCUMENT",
		"spdxVersion": "SPDX-2.3",
		"name": "test",
		"packages": [
			{
				"SPDXID": "SPDXRef-pkg1",
				"name": "adduser",
				"versionInfo": "3.134",
				"externalRefs": [
					{
						"referenceCategory": "PACKAGE_MANAGER",
						"referenceLocator": "pkg:deb/debian/adduser@3.134",
						"referenceType": "purl"
					}
				]
			},
			{
				"SPDXID": "SPDXRef-pkg2",
				"name": "apt",
				"versionInfo": "2.6.1",
				"externalRefs": [
					{
						"referenceCategory": "PACKAGE_MANAGER",
						"referenceLocator": "pkg:deb/debian/apt@2.6.1",
						"referenceType": "purl"
					}
				]
			}
		],
		"relationships": [
			{
				"spdxElementId": "SPDXRef-pkg2",
				"relationshipType": "DEPENDS_ON",
				"relatedSpdxElement": "SPDXRef-pkg1"
			}
		]
	}`

	path := writeTempFile(t, "test.spdx.json", content)
	bom, err := ParseBOM(path)
	if err != nil {
		t.Fatalf("ParseBOM() error = %v", err)
	}

	if bom.BomFormat != "CycloneDX" {
		t.Errorf("BomFormat = %v, want CycloneDX", bom.BomFormat)
	}

	if len(bom.Components) != 2 {
		t.Fatalf("got %d components, want 2", len(bom.Components))
	}

	// Check first component
	if bom.Components[0].Name != "adduser" {
		t.Errorf("Components[0].Name = %v, want adduser", bom.Components[0].Name)
	}
	if bom.Components[0].Purl != "pkg:deb/debian/adduser@3.134" {
		t.Errorf("Components[0].Purl = %v, want pkg:deb/debian/adduser@3.134", bom.Components[0].Purl)
	}
	if bom.Components[0].BomRef != "SPDXRef-pkg1" {
		t.Errorf("Components[0].BomRef = %v, want SPDXRef-pkg1", bom.Components[0].BomRef)
	}

	// Check dependency mapping: pkg2 DEPENDS_ON pkg1
	found := false
	for _, comp := range bom.Components {
		if comp.BomRef == "SPDXRef-pkg2" && len(comp.Dependencies) > 0 {
			if comp.Dependencies[0] == "SPDXRef-pkg1" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected SPDXRef-pkg2 to depend on SPDXRef-pkg1")
	}
}

func TestParseBOM_SkipsPackagesWithoutPURL(t *testing.T) {
	content := `{
		"SPDXID": "SPDXRef-DOCUMENT",
		"spdxVersion": "SPDX-2.3",
		"name": "test",
		"packages": [
			{
				"SPDXID": "SPDXRef-pkg1",
				"name": "no-purl-package",
				"versionInfo": "1.0"
			},
			{
				"SPDXID": "SPDXRef-pkg2",
				"name": "has-purl",
				"versionInfo": "2.0",
				"externalRefs": [
					{
						"referenceCategory": "PACKAGE_MANAGER",
						"referenceLocator": "pkg:npm/has-purl@2.0",
						"referenceType": "purl"
					}
				]
			}
		]
	}`

	path := writeTempFile(t, "test.spdx.json", content)
	bom, err := ParseBOM(path)
	if err != nil {
		t.Fatalf("ParseBOM() error = %v", err)
	}

	if len(bom.Components) != 1 {
		t.Fatalf("got %d components, want 1 (should skip package without PURL)", len(bom.Components))
	}

	if bom.Components[0].Name != "has-purl" {
		t.Errorf("Components[0].Name = %v, want has-purl", bom.Components[0].Name)
	}
}

func TestParseBOM_ReverseRelationships(t *testing.T) {
	content := `{
		"SPDXID": "SPDXRef-DOCUMENT",
		"spdxVersion": "SPDX-2.3",
		"name": "test",
		"packages": [
			{
				"SPDXID": "SPDXRef-app",
				"name": "app",
				"versionInfo": "1.0",
				"externalRefs": [{"referenceCategory": "PACKAGE_MANAGER", "referenceLocator": "pkg:npm/app@1.0", "referenceType": "purl"}]
			},
			{
				"SPDXID": "SPDXRef-junit",
				"name": "junit",
				"versionInfo": "4.0",
				"externalRefs": [{"referenceCategory": "PACKAGE_MANAGER", "referenceLocator": "pkg:maven/junit@4.0", "referenceType": "purl"}]
			}
		],
		"relationships": [
			{
				"spdxElementId": "SPDXRef-junit",
				"relationshipType": "TEST_DEPENDENCY_OF",
				"relatedSpdxElement": "SPDXRef-app"
			}
		]
	}`

	path := writeTempFile(t, "test.spdx.json", content)
	bom, err := ParseBOM(path)
	if err != nil {
		t.Fatalf("ParseBOM() error = %v", err)
	}

	// TEST_DEPENDENCY_OF is a reverse relationship: junit is a test dep OF app
	// So app depends on junit
	for _, comp := range bom.Components {
		if comp.BomRef == "SPDXRef-app" {
			if len(comp.Dependencies) == 0 {
				t.Error("expected SPDXRef-app to have dependencies (from TEST_DEPENDENCY_OF)")
			} else if comp.Dependencies[0] != "SPDXRef-junit" {
				t.Errorf("expected SPDXRef-app to depend on SPDXRef-junit, got %v", comp.Dependencies[0])
			}
			return
		}
	}
	t.Error("SPDXRef-app component not found")
}

func TestParseBOM_FileNotFound(t *testing.T) {
	_, err := ParseBOM("/nonexistent/path.spdx.json")
	if err == nil {
		t.Error("ParseBOM() expected error for nonexistent file")
	}
}

func TestParseBOM_InvalidJSON(t *testing.T) {
	path := writeTempFile(t, "bad.json", "not json")
	_, err := ParseBOM(path)
	if err == nil {
		t.Error("ParseBOM() expected error for invalid JSON")
	}
}

func TestParseBOM_RealExample(t *testing.T) {
	examplePath := filepath.Join("..", "..", "..", "examples", "spdx-debian-example.spdx.json")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Skip("example file not found, skipping")
	}

	bom, err := ParseBOM(examplePath)
	if err != nil {
		t.Fatalf("ParseBOM() error = %v", err)
	}

	// The debian example has 3 packages with PURLs (adduser, apt, bash)
	// The root debian-slim package has no PURL and should be skipped
	if len(bom.Components) != 3 {
		t.Errorf("got %d components, want 3", len(bom.Components))
	}

	// Check that PURLs were extracted correctly
	purls := make(map[string]bool)
	for _, comp := range bom.Components {
		purls[comp.Purl] = true
	}
	expectedPurls := []string{
		"pkg:deb/debian/adduser@3.134?arch=all&distro=debian-12",
		"pkg:deb/debian/apt@2.6.1?arch=amd64&distro=debian-12",
		"pkg:deb/debian/bash@5.2.15?arch=amd64&distro=debian-12",
	}
	for _, purl := range expectedPurls {
		if !purls[purl] {
			t.Errorf("expected PURL %s not found in components", purl)
		}
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	return path
}
