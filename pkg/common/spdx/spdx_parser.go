package spdx

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
)

// SPDX internal types for parsing

type Document struct {
	SPDXVersion   string         `json:"spdxVersion"`
	SPDXID        string         `json:"SPDXID"`
	Name          string         `json:"name"`
	Packages      []Package      `json:"packages"`
	Relationships []Relationship `json:"relationships"`
}

type Package struct {
	SPDXID       string        `json:"SPDXID"`
	Name         string        `json:"name"`
	VersionInfo  string        `json:"versionInfo"`
	ExternalRefs []ExternalRef `json:"externalRefs"`
}

type ExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceLocator  string `json:"referenceLocator"`
	ReferenceType     string `json:"referenceType"`
}

type Relationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

// dependencyRelationships are SPDX relationship types that map to dependency edges.
var dependencyRelationships = map[string]bool{
	"DEPENDS_ON":          true,
	"DYNAMIC_LINK":        true,
	"STATIC_LINK":         true,
	"CONTAINS":            true,
	"RUNTIME_DEPENDENCY_OF": true,
	"BUILD_DEPENDENCY_OF":  true,
	"DEV_DEPENDENCY_OF":    true,
	"TEST_DEPENDENCY_OF":   true,
}

// ParseBOM parses an SPDX JSON file and converts it to a cyclonedx.Bom
// so it can feed into the existing processing pipeline.
func ParseBOM(filename string) (*cyclonedx.Bom, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read SPDX file: %w", err)
	}

	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse SPDX JSON: %w", err)
	}

	// Build a map of SPDXID → dependency SPDXIDs from relationships
	depsMap := make(map[string][]string)
	for _, rel := range doc.Relationships {
		if dependencyRelationships[rel.RelationshipType] {
			// "A DEPENDS_ON B" means A depends on B
			// "B TEST_DEPENDENCY_OF A" means B is a dep of A → A depends on B
			if rel.RelationshipType == "DEPENDS_ON" || rel.RelationshipType == "DYNAMIC_LINK" ||
				rel.RelationshipType == "STATIC_LINK" || rel.RelationshipType == "CONTAINS" {
				depsMap[rel.SPDXElementID] = append(depsMap[rel.SPDXElementID], rel.RelatedSPDXElement)
			} else {
				// *_OF relationships: the related element depends on the source
				depsMap[rel.RelatedSPDXElement] = append(depsMap[rel.RelatedSPDXElement], rel.SPDXElementID)
			}
		}
	}

	// Convert packages to CycloneDX components
	var components []cyclonedx.Component
	for _, pkg := range doc.Packages {
		purl := extractPURL(pkg)
		if purl == "" {
			continue // Skip packages without PURLs (same as CycloneDX parser)
		}

		comp := cyclonedx.Component{
			Name:    pkg.Name,
			Version: pkg.VersionInfo,
			Purl:    purl,
			BomRef:  pkg.SPDXID,
		}

		if deps, ok := depsMap[pkg.SPDXID]; ok {
			comp.Dependencies = deps
		}

		components = append(components, comp)
	}

	// Build CycloneDX dependencies from the relationships
	var dependencies []cyclonedx.Dependency
	for ref, depRefs := range depsMap {
		dependencies = append(dependencies, cyclonedx.Dependency{
			Ref:       ref,
			DependsOn: depRefs,
		})
	}

	bom := &cyclonedx.Bom{
		BomFormat:    "CycloneDX",
		SpecVersion:  "1.4",
		Components:   components,
		Dependencies: dependencies,
	}

	return bom, nil
}

// extractPURL finds the PURL from a package's externalRefs.
func extractPURL(pkg Package) string {
	for _, ref := range pkg.ExternalRefs {
		if ref.ReferenceType == "purl" {
			return ref.ReferenceLocator
		}
	}
	return ""
}
