package cyclonedx

import (
	"encoding/json"
	"fmt"
	"os"
)

type Metadata struct {
	Component Component `json:"component"`
}

type Bom struct {
	BomFormat    string       `json:"bomFormat"`
	SpecVersion  string       `json:"specVersion"`
	Metadata     Metadata     `json:"metadata"`
	Components   []Component  `json:"components"`
	Dependencies []Dependency `json:"dependencies"`
}
type Component struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Purl         string   `json:"purl"`
	BomRef       string   `json:"bom-ref"`
	Dependencies []string `json:"dependsOn,omitempty"`
}

type Dependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn"`
}

func ParseBOM(filename string) (*Bom, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var bom Bom
	if err := json.Unmarshal(data, &bom); err != nil {
		return nil, err
	}

	dependenciesMap := make(map[string][]string)
	for _, dependency := range bom.Dependencies {
		dependenciesMap[dependency.Ref] = dependency.DependsOn
	}

	for i, component := range bom.Components {
		if deps, found := dependenciesMap[component.BomRef]; found {
			bom.Components[i].Dependencies = deps
		}
	}

	if err := validateBOM(&bom); err != nil {
		return nil, err
	}

	return &bom, nil
}

func validateBOM(bom *Bom) error {
	if bom.BomFormat != "CycloneDX" {
		return fmt.Errorf("invalid BOM format: %s", bom.BomFormat)
	}

	var filteredComponents []Component
	for _, component := range bom.Components {
		if component.Purl != "" {
			filteredComponents = append(filteredComponents, component)
		}
	}
	bom.Components = filteredComponents

	var filteredDependencies []Dependency
	for _, dependency := range bom.Dependencies {
		if dependency.Ref != "" && len(dependency.DependsOn) > 0 {
			filteredDependencies = append(filteredDependencies, dependency)
		}
	}
	bom.Dependencies = filteredDependencies

	if err := ExportBOM(bom, "exported_bom.json"); err != nil {
		fmt.Printf("Error exporting BOM: %v\n", err)
		return err
	}

	return nil
}

func ExportBOM(bom *Bom, filename string) error {
	data, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal BOM: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write BOM to file: %v", err)
	}

	// fmt.Printf("Exported file: %s to dir", filename)  // Debug output

	return nil
}
