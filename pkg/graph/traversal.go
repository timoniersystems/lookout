package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"timonier.systems/lookout/pkg/logging"
)

// DependsOn represents a dependency relationship
type DependsOn struct {
	Uid              string      `json:"uid"`
	Name             string      `json:"name"`
	Version          string      `json:"version"`
	Purl             string      `json:"purl"`
	Vulnerable       bool        `json:"vulnerable"`
	BomRef           string      `json:"bomRef"`
	Reference        string      `json:"reference"`
	DependsOn        []DependsOn `json:"dependsOn"`
	ReverseDependsOn []DependsOn `json:"~dependsOn"`
}

// Component represents a software component in the dependency graph
type Component struct {
	Uid              string      `json:"uid"`
	Name             string      `json:"name"`
	Version          string      `json:"version"`
	Purl             string      `json:"purl"`
	CveID            string      `json:"cveID"`
	Vulnerable       bool        `json:"vulnerable"`
	BomRef           string      `json:"bomRef"`
	Reference        string      `json:"reference"`
	Root             bool        `json:"root"`
	DependsOn        []DependsOn `json:"dependsOn"`
	ReverseDependsOn []DependsOn `json:"~dependsOn"`
}

// PathResult represents the result of a path search
type PathResult struct {
	SourceUID   string
	SourcePURL  string
	RootUID     string
	RootPURL    string
	Path        []string              // UIDs in path
	Components  []Component    // Full component details in path
	Depth       int                   // Depth at which path was found
	Found       bool                  // Whether a path exists
}

// GraphTraversal provides graph traversal operations
type GraphTraversal struct {
	client   DgraphClient
	maxDepth int
}

// NewGraphTraversal creates a new graph traversal instance
func NewGraphTraversal(client DgraphClient) *GraphTraversal {
	return &GraphTraversal{
		client:   client,
		maxDepth: 10, // Max BFS depth
	}
}

// FindShortestPath finds the shortest path from source to root
func (g *GraphTraversal) FindShortestPath(ctx context.Context, sourceUID, rootUID string, components map[string]Component) *PathResult {
	result := &PathResult{
		SourceUID: sourceUID,
		RootUID:   rootUID,
		Found:     false,
	}

	// Try increasing depths
	for depth := 1; depth < g.maxDepth; depth++ {
		select {
		case <-ctx.Done():
			return result
		default:
			logging.Debug("Searching with depth: %d", depth)
			path := g.bfsSearch(ctx, sourceUID, rootUID, components, depth)
			if path != nil {
				result.Path = path
				result.Components = g.extractComponents(components, path)
				result.Depth = depth
				result.Found = true

				// Get PURLs
				if src, ok := components[sourceUID]; ok {
					result.SourcePURL = src.Purl
				}
				if root, ok := components[rootUID]; ok {
					result.RootPURL = root.Purl
				}

				return result
			}
		}
	}

	return result
}

// bfsSearch performs BFS limited by depth
func (g *GraphTraversal) bfsSearch(ctx context.Context, sourceUID, rootUID string, components map[string]Component, maxDepth int) []string {
	type Node struct {
		UID   string
		Path  []string
		Depth int
	}

	visited := make(map[string]bool)
	queue := []Node{{UID: sourceUID, Path: []string{sourceUID}, Depth: 0}}

	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return nil
		default:
			node := queue[0]
			queue = queue[1:]

			if node.UID == rootUID {
				return node.Path
			}

			if node.Depth >= maxDepth {
				continue
			}

			visited[node.UID] = true
			currentComponent, exists := components[node.UID]
			if !exists {
				// Try to fetch this component
				if fetched := g.fetchComponent(ctx, node.UID); fetched != nil {
					components[node.UID] = *fetched
					currentComponent = *fetched
				} else {
					continue
				}
			}

			// Add forward dependencies
			for _, dep := range currentComponent.DependsOn {
				if !visited[dep.Uid] && dep.Name != "No dependencies found" {
					queue = append(queue, Node{
						UID:   dep.Uid,
						Path:  append(node.Path, dep.Uid),
						Depth: node.Depth + 1,
					})
				}
			}

			// Add reverse dependencies
			for _, dep := range currentComponent.ReverseDependsOn {
				if !visited[dep.Uid] && dep.Name != "No dependencies found" {
					queue = append(queue, Node{
						UID:   dep.Uid,
						Path:  append(node.Path, dep.Uid),
						Depth: node.Depth + 1,
					})
				}
			}
		}
	}

	return nil
}

// fetchComponent fetches a single component with dependencies from Dgraph
func (g *GraphTraversal) fetchComponent(ctx context.Context, uid string) *Component {
	query := fmt.Sprintf(`
	{
		component(func: uid(%s)) {
			uid
			name
			purl
			cveID
			version
			vulnerable
			bomRef
			reference
			root
			dependsOn {
				uid
				name
				purl
				version
				vulnerable
				bomRef
				reference
				root
			}
			~dependsOn {
				uid
				name
				purl
				version
				vulnerable
				bomRef
				reference
				root
			}
		}
	}
	`, uid)

	txn := g.client.NewTxn()
	resp, err := txn.Query(ctx, query)
	if err != nil {
		logging.Error("Failed to fetch component %s: %v", uid, err)
		return nil
	}

	var result struct {
		Component []Component `json:"component"`
	}
	if err := json.Unmarshal(resp.GetJson(), &result); err != nil {
		logging.Error("Failed to unmarshal component %s: %v", uid, err)
		return nil
	}

	if len(result.Component) > 0 {
		return &result.Component[0]
	}
	return nil
}

// extractComponents converts UIDs to full components
func (g *GraphTraversal) extractComponents(components map[string]Component, path []string) []Component {
	var result []Component
	seen := make(map[string]bool)

	for _, uid := range path {
		if comp, exists := components[uid]; exists && !seen[uid] {
			seen[uid] = true
			result = append(result, comp)
		}
	}

	return result
}

// FindRootComponent finds the root component in a component map
func FindRootComponent(components map[string]Component) (string, string) {
	for _, comp := range components {
		if comp.Root || (comp.Purl != "" && strings.Contains(comp.Purl, "@root")) {
			log.Printf("Root component found: UID: %s, Name: %s", comp.Uid, comp.Name)
			return comp.Uid, comp.Name
		}
	}
	return "", ""
}

// BuildComponentMap recursively builds a complete component map from query results
func BuildComponentMap(components []Component) map[string]Component {
	resultMap := make(map[string]Component)

	var addComponent func(Component)
	addComponent = func(comp Component) {
		if _, exists := resultMap[comp.Uid]; exists {
			return
		}
		resultMap[comp.Uid] = comp

		// Recursively add dependencies
		for _, dep := range comp.DependsOn {
			if dep.Name != "No dependencies found" {
				addComponent(Component{
					Uid:              dep.Uid,
					Name:             dep.Name,
					Purl:             dep.Purl,
					Version:          dep.Version,
					Vulnerable:       dep.Vulnerable,
					BomRef:           dep.BomRef,
					Reference:        dep.Reference,
					DependsOn:        dep.DependsOn,
					ReverseDependsOn: dep.ReverseDependsOn,
				})
			}
		}

		// Recursively add reverse dependencies
		for _, dep := range comp.ReverseDependsOn {
			if dep.Name != "No dependencies found" {
				addComponent(Component{
					Uid:              dep.Uid,
					Name:             dep.Name,
					Purl:             dep.Purl,
					Version:          dep.Version,
					Vulnerable:       dep.Vulnerable,
					BomRef:           dep.BomRef,
					Reference:        dep.Reference,
					DependsOn:        dep.DependsOn,
					ReverseDependsOn: dep.ReverseDependsOn,
				})
			}
		}
	}

	for _, comp := range components {
		addComponent(comp)
	}

	return resultMap
}
