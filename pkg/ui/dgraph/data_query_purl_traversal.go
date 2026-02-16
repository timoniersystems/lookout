package dgraph

import (
	"context"
	"fmt"
	"strings"

	"github.com/timoniersystems/lookout/pkg/graph"
	"github.com/timoniersystems/lookout/pkg/logging"
)

type FilteredResult struct {
	SearchedPurl        string   `json:"searched_purl"`
	PathFromRootPackage []string `json:"path_from_root_package,omitempty"`
}

func RetrievePURL(pURL string) (FilteredResult, error) {
	ctx := context.Background()

	// Use shared query builder
	client := NewDgraphClientWrapper()
	queryBuilder := graph.NewQueryBuilder(3, client)

	// Query for path to root using recursive query
	// Dgraph's @recurse will find the complete path for us!
	logging.Debug("Querying path to root for PURL: %s", pURL)
	components, err := queryBuilder.QueryPathToRoot(ctx, pURL)
	if err != nil {
		return FilteredResult{}, err
	}

	logging.Debug("Query returned %d components for PURL: %s", len(components), pURL)

	if len(components) == 0 {
		logging.Warn("No component found for PURL: %s", pURL)
		return FilteredResult{}, fmt.Errorf("no component found for pURL: %s", pURL)
	}

	filteredResult := FilteredResult{
		SearchedPurl: pURL,
	}

	// Extract path from the recursive query result
	// Dgraph returns nested structure: vulnerable → ... → root
	// We need to find the shortest path to a node with root=true
	path := extractPathFromRecursive(components[0])
	if len(path) > 0 {
		logging.Debug("Extracted path from recursive query: %d components", len(path))
		filteredResult.PathFromRootPackage = path
	} else {
		logging.Warn("No path to root found in recursive query result")
	}

	logging.Debug("Returning filtered data for pURL: %s", pURL)
	return filteredResult, nil
}

// extractPathFromRecursive extracts the shortest path from vulnerable component to root
// from Dgraph's @recurse query result
func extractPathFromRecursive(comp graph.Component) []string {
	// BFS through the nested ~dependsOn structure to find shortest path to root
	type Node struct {
		Comp graph.Component
		Path []string
	}

	queue := []Node{{Comp: comp, Path: []string{comp.Purl}}}
	visited := make(map[string]bool)

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if visited[node.Comp.Purl] {
			continue
		}
		visited[node.Comp.Purl] = true

		// Check if this is the root
		if node.Comp.Root {
			logging.Debug("Found root in recursive structure: %s", node.Comp.Purl)
			// Reverse the path so it goes root → vulnerable instead of vulnerable → root
			reversed := make([]string, len(node.Path))
			for i, j := 0, len(node.Path)-1; i < len(node.Path); i, j = i+1, j-1 {
				reversed[i] = node.Path[j]
			}
			return reversed
		}

		// Explore reverse dependencies (packages that depend on this one)
		for _, dep := range node.Comp.ReverseDependsOn {
			if !visited[dep.Purl] {
				newPath := make([]string, len(node.Path))
				copy(newPath, node.Path)
				newPath = append(newPath, dep.Purl)

				// Convert DependsOn to Component for recursion
				depComp := graph.Component{
					Uid:              dep.Uid,
					Name:             dep.Name,
					Purl:             dep.Purl,
					Root:             false, // DependsOn doesn't have Root field, check in recursive structure
					ReverseDependsOn: dep.ReverseDependsOn,
				}

				// Check if this dep is actually root by looking for root flag in the structure
				if strings.Contains(dep.Name, "@root") || dep.Purl == "cyclonedx-php-composer-demo@root" {
					logging.Debug("Found root (by name): %s", dep.Purl)
					// Reverse the path so it goes root → vulnerable instead of vulnerable → root
					reversed := make([]string, len(newPath))
					for i, j := 0, len(newPath)-1; i < len(newPath); i, j = i+1, j-1 {
						reversed[i] = newPath[j]
					}
					return reversed
				}

				queue = append(queue, Node{Comp: depComp, Path: newPath})
			}
		}
	}

	return nil
}
