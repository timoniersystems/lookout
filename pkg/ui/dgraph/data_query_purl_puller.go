package dgraph

import (
	"log"

	"github.com/timoniersystems/lookout/pkg/graph"
)

// Type aliases to use graph types
type (
	DependsOn  = graph.DependsOn
	Component  = graph.Component
	QueryResult struct {
		Component []Component `json:"component"`
	}
)

func RetrieveVulnerablePURLs(cvePurlMap map[string]string) (map[string]Component, error) {
	resultMap := make(map[string]Component)

	// Use the shared RetrievePURL function for each CVE+PURL pair
	for cveID, purl := range cvePurlMap {
		log.Printf("Querying for CVE: %s and PURL: %s", cveID, purl)

		// Call the shared function that both CLI and UI use
		filteredResult, err := RetrievePURL(purl)
		if err != nil {
			log.Printf("Failed to retrieve path for CVE: %s and PURL: %s: %v", cveID, purl, err)
			// Don't fail the entire request, just skip this one
			continue
		}

		// Convert FilteredResult to Component format for UI
		// The UI expects a Component with DependsOn populated with the path
		component := Component{
			Purl:       purl,
			Vulnerable: true,
		}

		// Convert path to DependsOn format
		// The path already goes from root → vulnerable, so we need to reverse it for the UI
		// because the UI template reverses it back before displaying
		if len(filteredResult.PathFromRootPackage) > 0 {
			// Verbose debug logging - uncomment if needed for troubleshooting
			// log.Printf("PathFromRootPackage for %s: %v", purl, filteredResult.PathFromRootPackage)

			var pathDeps []DependsOn
			// Reverse iteration: start from second-to-last element (skip vulnerable at end)
			// and go down to first element (include root at beginning)
			// PathFromRootPackage = [root, intermediate..., vulnerable]
			// We want: [intermediate..., root] which template will reverse to [root, intermediate...]
			for i := len(filteredResult.PathFromRootPackage) - 2; i >= 0; i-- {
				purl := filteredResult.PathFromRootPackage[i]
				// log.Printf("  Adding to pathDeps[%d]: %s", i, purl)
				pathDeps = append(pathDeps, DependsOn{
					Purl: purl,
				})
			}

			// Verbose debug logging - uncomment if needed for troubleshooting
			// log.Printf("Final pathDeps (before template reverse): %d elements", len(pathDeps))
			// for idx, dep := range pathDeps {
			// 	log.Printf("  pathDeps[%d]: %s", idx, dep.Purl)
			// }

			component.DependsOn = pathDeps
			log.Printf("Found path with %d components for CVE: %s and PURL: %s", len(pathDeps), cveID, purl)
		} else {
			// No path found - create a "No dependencies found" entry
			component.DependsOn = []DependsOn{{Name: "No dependencies found"}}
			log.Printf("No path found for CVE: %s and PURL: %s", cveID, purl)
		}

		// Use PURL as the key in the result map
		resultMap[purl] = component
	}

	return resultMap, nil
}
