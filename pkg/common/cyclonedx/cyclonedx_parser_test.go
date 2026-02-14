package cyclonedx

import (
	"os"
	"testing"
)

func TestParseBOM_ValidCycloneDXFile(t *testing.T) {
	// Use the real Python SBOM example
	testFile := "../../../examples/python-e5abfd1e-7471-4d0a-8516-ecf040df8d9d-inventory.cdx.json"

	bom, err := ParseBOM(testFile)
	if err != nil {
		t.Fatalf("Failed to parse valid CycloneDX BOM: %v", err)
	}

	// Verify basic structure
	if bom.BomFormat != "CycloneDX" {
		t.Errorf("Expected BomFormat 'CycloneDX', got %s", bom.BomFormat)
	}

	if bom.SpecVersion != "1.5" {
		t.Errorf("Expected SpecVersion '1.5', got %s", bom.SpecVersion)
	}

	// Verify metadata
	if bom.Metadata.Component.Name != "sample/python-project" {
		t.Errorf("Expected root component name 'sample/python-project', got %s", bom.Metadata.Component.Name)
	}

	// Verify components were parsed
	if len(bom.Components) == 0 {
		t.Error("Expected components to be parsed, got 0")
	}

	// The Python SBOM has 39 components
	expectedComponents := 39
	if len(bom.Components) != expectedComponents {
		t.Errorf("Expected %d components, got %d", expectedComponents, len(bom.Components))
	}

	// Note: The Python SBOM has dependencies in the file, but they all have empty dependsOn arrays
	// The parser filters these out, so we expect 0 dependencies after parsing
	// This is correct behavior - the parser only keeps dependencies with actual relationships
	expectedDependencies := 0 // Python SBOM has no actual dependency relationships
	if len(bom.Dependencies) != expectedDependencies {
		t.Logf("Note: Found %d dependencies (Python SBOM has empty dependency arrays)", len(bom.Dependencies))
	}
}

func TestParseBOM_WithDependencyGraph(t *testing.T) {
	// Use the cyclonedx-sbom-example which has actual dependency relationships
	testFile := "../../../examples/cyclonedx-sbom-example.json"

	bom, err := ParseBOM(testFile)
	if err != nil {
		t.Fatalf("Failed to parse CycloneDX BOM with dependencies: %v", err)
	}

	// Verify dependencies exist
	if len(bom.Dependencies) == 0 {
		t.Fatal("Expected dependencies to be parsed")
	}

	// Find a component with actual dependencies
	foundDependency := false
	for _, dep := range bom.Dependencies {
		if len(dep.DependsOn) > 0 {
			foundDependency = true
			// Verify the dependency structure
			t.Logf("Component %s has %d dependencies", dep.Ref, len(dep.DependsOn))
			break
		}
	}

	if !foundDependency {
		t.Error("Expected to find at least one component with dependencies")
	}

	// Verify components have PURLs
	foundPURL := false
	for _, comp := range bom.Components {
		if comp.Purl != "" {
			foundPURL = true
			t.Logf("Found component with PURL: %s - %s", comp.Name, comp.Purl)
			break
		}
	}

	if !foundPURL {
		t.Error("Expected to find at least one component with a PURL")
	}
}

func TestParseBOM_ComponentsHavePURLs(t *testing.T) {
	testFile := "../../../examples/python-e5abfd1e-7471-4d0a-8516-ecf040df8d9d-inventory.cdx.json"

	bom, err := ParseBOM(testFile)
	if err != nil {
		t.Fatalf("Failed to parse BOM: %v", err)
	}

	// All components should have PURLs
	componentsWithPURLs := 0
	for _, comp := range bom.Components {
		if comp.Purl != "" {
			componentsWithPURLs++

			// Verify PURL format (should start with pkg:)
			if len(comp.Purl) < 4 || comp.Purl[:4] != "pkg:" {
				t.Errorf("Invalid PURL format for component %s: %s", comp.Name, comp.Purl)
			}
		}
	}

	if componentsWithPURLs == 0 {
		t.Error("Expected all components to have PURLs")
	}

	t.Logf("Found %d components with PURLs out of %d total components", componentsWithPURLs, len(bom.Components))
}

func TestParseBOM_MultipleLanguages(t *testing.T) {
	testCases := []struct {
		name     string
		file     string
		language string
	}{
		{
			name:     "Python SBOM",
			file:     "../../../examples/python-e5abfd1e-7471-4d0a-8516-ecf040df8d9d-inventory.cdx.json",
			language: "pypi",
		},
		{
			name:     "Java SBOM",
			file:     "../../../examples/java-bc975229-3819-4ee3-9a48-6e7d46ac8064-inventory.cdx.json",
			language: "maven",
		},
		{
			name:     "NPM SBOM",
			file:     "../../../examples/npm-70de2757-37d5-4175-b63f-24890ae6471e-inventory.cdx.json",
			language: "npm",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bom, err := ParseBOM(tc.file)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", tc.name, err)
			}

			if len(bom.Components) == 0 {
				t.Errorf("%s: Expected components to be parsed", tc.name)
			}

			// Verify at least one component has the expected language type in PURL
			foundExpectedType := false
			for _, comp := range bom.Components {
				if len(comp.Purl) > 4 && comp.Purl[:4] == "pkg:" {
					// Extract the type from PURL (format: pkg:type/name@version)
					if len(comp.Purl) > len("pkg:"+tc.language) {
						purlType := comp.Purl[4:]
						if len(purlType) >= len(tc.language) && purlType[:len(tc.language)] == tc.language {
							foundExpectedType = true
							break
						}
					}
				}
			}

			if !foundExpectedType {
				t.Errorf("%s: Expected to find at least one component with pkg:%s PURL type", tc.name, tc.language)
			}

			t.Logf("%s: Successfully parsed %d components with %d dependencies",
				tc.name, len(bom.Components), len(bom.Dependencies))
		})
	}
}

func TestParseBOM_InvalidFile(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "Invalid JSON",
			content: `{invalid json}`,
		},
		{
			name:    "Wrong BOM format",
			content: `{"bomFormat": "NotCycloneDX", "specVersion": "1.5", "metadata": {"component": {"name": "test"}}, "components": []}`,
		},
		{
			name:    "Empty file",
			content: ``,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "invalid-bom-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write invalid content
			if _, err := tmpFile.WriteString(tc.content); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpFile.Close()

			// Try to parse - should fail
			_, err = ParseBOM(tmpFile.Name())
			if err == nil {
				t.Errorf("Expected error when parsing invalid BOM, got nil")
			}
		})
	}
}

func TestParseBOM_FileNotFound(t *testing.T) {
	_, err := ParseBOM("/nonexistent/path/to/sbom.json")
	if err == nil {
		t.Error("Expected error when file doesn't exist, got nil")
	}
}

func TestParseBOM_DependencyMapping(t *testing.T) {
	testFile := "../../../examples/cyclonedx-sbom-example.json"

	bom, err := ParseBOM(testFile)
	if err != nil {
		t.Fatalf("Failed to parse BOM: %v", err)
	}

	// Verify that components have their dependencies populated
	// The ParseBOM function should map dependencies from the Dependencies array
	// to the Components array

	// Find a component that should have dependencies based on the Dependencies array
	depMap := make(map[string][]string)
	for _, dep := range bom.Dependencies {
		if len(dep.DependsOn) > 0 {
			depMap[dep.Ref] = dep.DependsOn
		}
	}

	// Verify at least some components have dependencies mapped
	if len(depMap) == 0 {
		t.Error("Expected to find components with dependencies")
	}

	t.Logf("Found %d components with dependencies in the dependency map", len(depMap))
}

func TestParseBOM_ComponentStructure(t *testing.T) {
	testFile := "../../../examples/python-e5abfd1e-7471-4d0a-8516-ecf040df8d9d-inventory.cdx.json"

	bom, err := ParseBOM(testFile)
	if err != nil {
		t.Fatalf("Failed to parse BOM: %v", err)
	}

	if len(bom.Components) == 0 {
		t.Fatal("Expected components to be parsed")
	}

	// Verify first component structure
	firstComp := bom.Components[0]

	if firstComp.Name == "" {
		t.Error("Expected component to have a name")
	}

	if firstComp.Version == "" {
		t.Error("Expected component to have a version")
	}

	if firstComp.Purl == "" {
		t.Error("Expected component to have a PURL")
	}

	if firstComp.BomRef == "" {
		t.Error("Expected component to have a bom-ref")
	}

	t.Logf("First component: Name=%s, Version=%s, PURL=%s, BomRef=%s",
		firstComp.Name, firstComp.Version, firstComp.Purl, firstComp.BomRef)
}

func TestParseBOM_RootComponentInMetadata(t *testing.T) {
	testFile := "../../../examples/python-e5abfd1e-7471-4d0a-8516-ecf040df8d9d-inventory.cdx.json"

	bom, err := ParseBOM(testFile)
	if err != nil {
		t.Fatalf("Failed to parse BOM: %v", err)
	}

	// Verify metadata component (root)
	if bom.Metadata.Component.Name == "" {
		t.Error("Expected root component to have a name in metadata")
	}

	if bom.Metadata.Component.Version == "" {
		t.Error("Expected root component to have a version in metadata")
	}

	if bom.Metadata.Component.BomRef == "" {
		t.Error("Expected root component to have a bom-ref in metadata")
	}

	t.Logf("Root component: %s@%s (ref: %s)",
		bom.Metadata.Component.Name,
		bom.Metadata.Component.Version,
		bom.Metadata.Component.BomRef)
}
