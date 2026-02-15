//go:build integration

// Package cyclonedx integration tests verify end-to-end SBOM processing and dependency traversal
package cyclonedx_test

import (
	"os"
	"testing"
	"time"

	"timonier.systems/lookout/pkg/common/cyclonedx"
	"timonier.systems/lookout/pkg/ui/dgraph"
)

// TestSBOMParsing_RealWorldExamples tests parsing of real-world SBOM files
func TestSBOMParsing_RealWorldExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testCases := []struct {
		name                string
		file                string
		expectedComponents  int
		expectedDependencies int
		hasActualDeps       bool // Whether the SBOM has non-empty dependency relationships
	}{
		{
			name:                "Python Project SBOM",
			file:                "../../../examples/python-e5abfd1e-7471-4d0a-8516-ecf040df8d9d-inventory.cdx.json",
			expectedComponents:  39,
			expectedDependencies: 0, // Python SBOM has empty dependency arrays, filtered out by parser
			hasActualDeps:       false,
		},
		{
			name:                "Laravel/PHP Project SBOM",
			file:                "../../../examples/cyclonedx-sbom-example.json",
			expectedComponents:  -1, // Don't check exact count
			expectedDependencies: -1,
			hasActualDeps:       true, // This one has real dependency relationships
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Check if file exists
			fullPath := tc.file
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Skipf("SBOM file not found: %s", fullPath)
				return
			}

			// Parse the SBOM
			bom, err := cyclonedx.ParseBOM(fullPath)
			if err != nil {
				t.Fatalf("Failed to parse SBOM: %v", err)
			}

			// Verify component count if specified
			if tc.expectedComponents > 0 {
				if len(bom.Components) != tc.expectedComponents {
					t.Errorf("Expected %d components, got %d",
						tc.expectedComponents, len(bom.Components))
				}
			}

			// Verify dependency count if specified
			if tc.expectedDependencies > 0 {
				if len(bom.Dependencies) != tc.expectedDependencies {
					t.Errorf("Expected %d dependencies, got %d",
						tc.expectedDependencies, len(bom.Dependencies))
				}
			}

			// Verify actual dependency relationships if expected
			if tc.hasActualDeps {
				foundRealDeps := false
				for _, dep := range bom.Dependencies {
					if len(dep.DependsOn) > 0 {
						foundRealDeps = true
						t.Logf("Found component %s with %d dependencies",
							dep.Ref, len(dep.DependsOn))
						break
					}
				}
				if !foundRealDeps {
					t.Error("Expected to find components with actual dependency relationships")
				}
			}

			// Verify all components have required fields
			for i, comp := range bom.Components {
				if comp.Name == "" {
					t.Errorf("Component %d missing name", i)
				}
				if comp.BomRef == "" {
					t.Errorf("Component %d (%s) missing bom-ref", i, comp.Name)
				}
			}

			t.Logf("%s: Successfully parsed with %d components and %d dependency entries",
				tc.name, len(bom.Components), len(bom.Dependencies))
		})
	}
}

// TestDependencyGraphStructure tests the structure of dependency graphs
func TestDependencyGraphStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use the Laravel SBOM which has actual dependency relationships
	sbomPath := "../../../examples/cyclonedx-sbom-example.json"

	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		t.Skipf("SBOM file not found: %s", sbomPath)
		return
	}

	bom, err := cyclonedx.ParseBOM(sbomPath)
	if err != nil {
		t.Fatalf("Failed to parse SBOM: %v", err)
	}

	// Build a map of component refs for quick lookup
	componentRefs := make(map[string]bool)
	for _, comp := range bom.Components {
		componentRefs[comp.BomRef] = true
	}

	// Verify dependency integrity
	invalidDeps := 0
	validDeps := 0
	for _, dep := range bom.Dependencies {
		for _, depRef := range dep.DependsOn {
			if !componentRefs[depRef] {
				invalidDeps++
				t.Logf("Warning: Dependency %s references non-existent component %s",
					dep.Ref, depRef)
			} else {
				validDeps++
			}
		}
	}

	t.Logf("Dependency integrity: %d valid, %d invalid references", validDeps, invalidDeps)

	// Analyze dependency depth
	maxDepth := 0
	totalDepth := 0
	componentsWithDeps := 0

	for _, dep := range bom.Dependencies {
		if len(dep.DependsOn) > 0 {
			componentsWithDeps++
			totalDepth += len(dep.DependsOn)
			if len(dep.DependsOn) > maxDepth {
				maxDepth = len(dep.DependsOn)
			}
		}
	}

	if componentsWithDeps > 0 {
		avgDepth := float64(totalDepth) / float64(componentsWithDeps)
		t.Logf("Dependency statistics: %d components have dependencies", componentsWithDeps)
		t.Logf("Max dependencies: %d, Average: %.2f", maxDepth, avgDepth)
	}
}

// TestVulnerabilityTraversalConcept tests the conceptual workflow of vulnerability traversal
func TestVulnerabilityTraversalConcept(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test verifies the conceptual workflow without requiring Dgraph
	// It demonstrates how the traversal should work

	sbomPath := "../../../examples/cyclonedx-sbom-example.json"

	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		t.Skipf("SBOM file not found: %s", sbomPath)
		return
	}

	bom, err := cyclonedx.ParseBOM(sbomPath)
	if err != nil {
		t.Fatalf("Failed to parse SBOM: %v", err)
	}

	// Step 1: Identify root component
	rootRef := bom.Metadata.Component.BomRef
	t.Logf("Root component: %s (ref: %s)", bom.Metadata.Component.Name, rootRef)

	// Step 2: Build dependency map for traversal simulation
	depMap := make(map[string][]string)
	reverseDeps := make(map[string][]string)

	for _, dep := range bom.Dependencies {
		depMap[dep.Ref] = dep.DependsOn
		// Build reverse dependency map (for finding paths TO root)
		for _, depRef := range dep.DependsOn {
			reverseDeps[depRef] = append(reverseDeps[depRef], dep.Ref)
		}
	}

	// Step 3: Find a leaf component (one that nothing depends on)
	leafComponents := []string{}
	for _, comp := range bom.Components {
		if len(depMap[comp.BomRef]) == 0 {
			leafComponents = append(leafComponents, comp.BomRef)
		}
	}

	t.Logf("Found %d leaf components (components with no dependencies)", len(leafComponents))

	// Step 4: Simulate finding a path from a leaf to root
	if len(leafComponents) > 0 {
		testLeaf := leafComponents[0]
		t.Logf("Testing traversal from leaf component: %s", testLeaf)

		// Simple BFS to find path to root
		path := findPathToRoot(testLeaf, rootRef, reverseDeps)
		if len(path) > 0 {
			t.Logf("Found path of length %d from %s to root %s",
				len(path), testLeaf, rootRef)
			t.Logf("Path: %v", path)
		} else {
			t.Logf("No path found from %s to root (this is expected for some SBOMs)", testLeaf)
		}
	}

	// Step 5: Demonstrate vulnerability scenario
	// In real usage, Trivy would identify vulnerable PURLs
	// Then we'd find the path from that vulnerable component to root
	componentsWithPURLs := 0
	for _, comp := range bom.Components {
		if comp.Purl != "" {
			componentsWithPURLs++
		}
	}

	t.Logf("Vulnerability mapping: %d components have PURLs for CVE correlation",
		componentsWithPURLs)

	t.Log("\nWorkflow summary:")
	t.Log("1. Parse CycloneDX SBOM ✓")
	t.Log("2. Build dependency graph ✓")
	t.Log("3. Identify components with PURLs ✓")
	t.Log("4. (Would) Run Trivy to find CVEs")
	t.Log("5. (Would) Map CVEs to PURLs")
	t.Log("6. (Would) Find traversal path from vulnerable component to root")
	t.Log("7. (Would) Display visual and text-based output of path")
}

// Helper function to find path using BFS
func findPathToRoot(start, root string, reverseDeps map[string][]string) []string {
	if start == root {
		return []string{start}
	}

	visited := make(map[string]bool)
	queue := [][]string{{start}}
	visited[start] = true

	maxIterations := 1000 // Prevent infinite loops
	iterations := 0

	for len(queue) > 0 && iterations < maxIterations {
		iterations++
		path := queue[0]
		queue = queue[1:]

		current := path[len(path)-1]

		// Check all components that depend on current
		for _, dependent := range reverseDeps[current] {
			if dependent == root {
				// Found the root!
				return append(path, dependent)
			}

			if !visited[dependent] {
				visited[dependent] = true
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = dependent
				queue = append(queue, newPath)
			}
		}
	}

	return nil // No path found
}

// TestComponentDataStructure verifies the dgraph.Component structure
func TestComponentDataStructure(t *testing.T) {
	// This tests the structure we use for representing components in Dgraph
	component := dgraph.Component{
		Uid:              "0x1",
		Name:             "express",
		Version:          "4.17.1",
		Purl:             "pkg:npm/express@4.17.1",
		CveID:            "CVE-2022-24999",
		Vulnerable:       true,
		BomRef:           "express-4.17.1",
		Reference:        "express",
		Root:             false,
		DependsOn:        []dgraph.DependsOn{},
		ReverseDependsOn: []dgraph.DependsOn{},
	}

	// Verify required fields
	if component.Name == "" {
		t.Error("Component name should not be empty")
	}
	if component.Purl == "" {
		t.Error("Component PURL should not be empty")
	}
	if component.DependsOn == nil {
		t.Error("DependsOn should be initialized")
	}
	if component.ReverseDependsOn == nil {
		t.Error("ReverseDependsOn should be initialized")
	}

	t.Logf("Component structure verified: %s@%s (%s)", component.Name, component.Version, component.Purl)
}

// TestTextBasedTraversalOutput demonstrates the text-based output format
func TestTextBasedTraversalOutput(t *testing.T) {
	// This test demonstrates what the text-based traversal output should look like
	// In the actual application, this would be displayed in the GUI or CLI

	// Simulate a vulnerability path
	vulnerablePath := []struct {
		name    string
		version string
		purl    string
		cveID   string
	}{
		{"express", "4.17.1", "pkg:npm/express@4.17.1", "CVE-2022-24999"},
		{"body-parser", "1.19.0", "pkg:npm/body-parser@1.19.0", ""},
		{"app", "1.0.0", "pkg:npm/app@1.0.0", ""},
	}

	t.Log("\n===== Reverse Vulnerability Traversal (Text Format) =====")
	t.Log("\nVulnerability: CVE-2022-24999")
	t.Log("Vulnerable Component: express@4.17.1")
	t.Log("\nPath from vulnerable component to root application:")
	t.Log("")

	for i, comp := range vulnerablePath {
		indent := ""
		for j := 0; j < i; j++ {
			indent += "  "
		}

		marker := "└─"
		if i == 0 {
			marker = "🔴" // Red circle for vulnerable component
		} else if i == len(vulnerablePath)-1 {
			marker = "🏠" // House for root
		} else {
			marker = "├─"
		}

		status := ""
		if comp.cveID != "" {
			status = " [VULNERABLE]"
		}

		t.Logf("%s%s %s@%s%s", indent, marker, comp.name, comp.version, status)
		if comp.purl != "" {
			t.Logf("%s   PURL: %s", indent, comp.purl)
		}
	}

	t.Log("\n=======================================================")
	t.Log("This demonstrates the text-based output format for vulnerability traversal")
}

// TestVisualTraversalDataPreparation tests preparing data for visual output
func TestVisualTraversalDataPreparation(t *testing.T) {
	// This test demonstrates how data should be prepared for visual graph display
	// The actual visualization would be done in the GUI using a graph library

	type GraphNode struct {
		ID         string
		Label      string
		Vulnerable bool
		IsRoot     bool
		CVE        string
	}

	type GraphEdge struct {
		From string
		To   string
		Type string
	}

	// Simulate preparing graph data
	nodes := []GraphNode{
		{ID: "1", Label: "express@4.17.1", Vulnerable: true, CVE: "CVE-2022-24999"},
		{ID: "2", Label: "body-parser@1.19.0", Vulnerable: false},
		{ID: "3", Label: "app@1.0.0", Vulnerable: false, IsRoot: true},
	}

	edges := []GraphEdge{
		{From: "1", To: "2", Type: "depends_on"},
		{From: "2", To: "3", Type: "depends_on"},
	}

	// Verify structure
	if len(nodes) == 0 {
		t.Error("Expected nodes to be populated")
	}
	if len(edges) == 0 {
		t.Error("Expected edges to be populated")
	}

	// Find vulnerable nodes
	vulnerableCount := 0
	rootCount := 0
	for _, node := range nodes {
		if node.Vulnerable {
			vulnerableCount++
			t.Logf("Vulnerable node found: %s (CVE: %s)", node.Label, node.CVE)
		}
		if node.IsRoot {
			rootCount++
			t.Logf("Root node: %s", node.Label)
		}
	}

	if vulnerableCount == 0 {
		t.Error("Expected at least one vulnerable node")
	}
	if rootCount != 1 {
		t.Error("Expected exactly one root node")
	}

	t.Log("\nVisual graph data prepared:")
	t.Logf("- %d nodes (%d vulnerable, %d root)", len(nodes), vulnerableCount, rootCount)
	t.Logf("- %d edges (dependency relationships)", len(edges))
	t.Log("This data would be used to render a visual dependency graph in the GUI")
}

// TestPerformanceWithLargeSBOM tests handling of larger SBOMs
func TestPerformanceWithLargeSBOM(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Use the largest SBOM example (NPM)
	sbomPath := "../../../examples/npm-70de2757-37d5-4175-b63f-24890ae6471e-inventory.cdx.json"

	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		t.Skipf("Large SBOM file not found: %s", sbomPath)
		return
	}

	start := time.Now()
	bom, err := cyclonedx.ParseBOM(sbomPath)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to parse large SBOM: %v", err)
	}

	t.Logf("Parsed large SBOM with %d components in %v", len(bom.Components), elapsed)

	if elapsed > 5*time.Second {
		t.Errorf("Parsing took too long: %v (expected < 5s)", elapsed)
	}

	// Test dependency graph building performance
	start = time.Now()
	depMap := make(map[string][]string)
	for _, dep := range bom.Dependencies {
		depMap[dep.Ref] = dep.DependsOn
	}
	elapsed = time.Since(start)

	t.Logf("Built dependency map for %d components in %v", len(depMap), elapsed)

	if elapsed > 1*time.Second {
		t.Errorf("Building dependency map took too long: %v (expected < 1s)", elapsed)
	}
}
