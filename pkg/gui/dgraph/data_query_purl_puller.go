package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type DependsOn struct {
	Uid        string `json:"uid"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Purl       string `json:"purl"`
	Vulnerable bool   `json:"vulnerable"`
	BomRef     string `json:"bomRef"`
	Reference  string `json:"reference"`
}

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

type QueryResult struct {
	Component []Component `json:"component"`
}

func RetrieveVulnerablePURLs(cvePurlMap map[string]string) (map[string]Component, error) {
	client := DgraphClient()
	ctx := context.Background()

	query := `
	query PurlAndCveQuery($purl: string, $cveID: string) {
		component(func: eq(purl, $purl)) @filter(eq(cveID, $cveID)) {
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
				cveID
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
				cveID
				version
				vulnerable
				bomRef
				reference
				root
			}
		}
	}
	`

	resultMap := make(map[string]Component)

	for cveID, purl := range cvePurlMap {
		log.Printf("Querying for CVE: %s and PURL: %s", cveID, purl)
		variables := map[string]string{"$purl": purl, "$cveID": cveID}

		txn := client.NewTxn()
		defer txn.Discard(ctx)

		resp, err := txn.QueryWithVars(context.Background(), query, variables)
		if err != nil {
			log.Printf("Query failed for CVE: %s and PURL: %s", cveID, purl)
			return nil, err
		}

		var result QueryResult
		if err := json.Unmarshal(resp.GetJson(), &result); err != nil {
			log.Printf("Failed to unmarshal response for CVE: %s and PURL: %s", cveID, purl)
			return nil, err
		}

		log.Printf("Here is the QueryResult: %s", result)

		for _, component := range result.Component {
			//THIS IS TESTING. WILL REMOVE IF ERROR IS SHOWN. - JENSEN
			if len(component.DependsOn) == 0 {
				component.DependsOn = []DependsOn{{Name: "No dependencies found"}}
			}
			if len(component.ReverseDependsOn) == 0 {
				component.ReverseDependsOn = []DependsOn{{Name: "No dependencies found"}}
			}
			//THIS ABOVE IS TESTING. WILL REMOVE IF ERROR IS SHOWN. - JENSEN
			resultMap[component.Uid] = component
			log.Printf("Component found: UID: %s, Name: %s, PURL: %s", component.Uid, component.Name, component.Purl, component.CveID)
		}
	}

	rootQuery := `
	{
		component(func: eq(root, true)) {
			uid
			name
			purl
			cveID
			version
			vulnerable
			bomRef
			reference
			root
		}
	}
	`

	rootTxn := client.NewTxn()
	defer rootTxn.Discard(ctx)

	rootResp, err := rootTxn.Query(context.Background(), rootQuery)
	if err != nil {
		log.Printf("Query failed for root component")
		return nil, err
	}

	var rootResult QueryResult
	if err := json.Unmarshal(rootResp.GetJson(), &rootResult); err != nil {
		log.Printf("Failed to unmarshal response for root component")
		return nil, err
	}

	for _, component := range rootResult.Component {
		if len(component.DependsOn) == 0 {
			component.DependsOn = []DependsOn{{Name: "No dependencies found"}}
		}
		if len(component.ReverseDependsOn) == 0 {
			component.ReverseDependsOn = []DependsOn{{Name: "No dependencies found"}}
		}
		resultMap[component.Uid] = component
		log.Printf("Root component found: UID: %s, Name: %s, PURL: %s", component.Uid, component.Name, component.Purl)
	}

	rootUID, rootName := findRootUID(resultMap)

	if rootUID == "" {
		log.Printf("Root component not found")
		return resultMap, nil
	}

	log.Printf("Root UID: %s, Root Name: %s", rootUID, rootName)

	for uid, component := range resultMap {
		if component.Purl == "" || !strings.Contains(component.Purl, "@root") {
			log.Printf("Finding shortest path for component: %s, Name: %s", component.Uid, component.Name)
			path := FindShortestPathWithDepth(resultMap, uid, rootUID)
			if path != nil {
				comp := resultMap[component.Uid]
				comp.DependsOn = extractPathComponents(resultMap, path)
				resultMap[component.Uid] = comp
				log.Printf("Shortest path for component %s (%s): %v", component.Uid, component.Name, path)
			} else {
				log.Printf("No path found for component %s (%s) to root %s (%s)", component.Uid, component.Name, rootUID, rootName)
			}
		}
	}

	return resultMap, nil
}

func findRootUID(components map[string]Component) (string, string) {
	for _, comp := range components {
		if comp.Root || (comp.Purl != "" && strings.Contains(comp.Purl, "@root")) {
			log.Printf("Root component found: UID: %s, Name: %s", comp.Uid, comp.Name)
			return comp.Uid, comp.Name
		}
	}
	return "", ""
}

func extractPathComponents(components map[string]Component, path []string) []DependsOn {
	var result []DependsOn
	seen := make(map[string]bool)
	for _, uid := range path {
		if comp, exists := components[uid]; exists && !seen[uid] {
			seen[uid] = true
			result = append(result, DependsOn{
				Uid:        comp.Uid,
				Name:       comp.Name,
				Version:    comp.Version,
				Purl:       comp.Purl,
				Vulnerable: comp.Vulnerable,
				BomRef:     comp.BomRef,
				Reference:  comp.Reference,
			})
		}
	}
	return result
}

func FindShortestPathWithDepth(components map[string]Component, sourceUID, rootUID string) []string {
	depth := 1
	for depth < 10 {
		log.Printf("Searching with depth: %d", depth)
		path := FindShortestPathWithDepthHelper(components, sourceUID, rootUID, depth)
		if path != nil {
			return path
		}
		depth++
	}
	return nil
}

func FindShortestPathWithDepthHelper(components map[string]Component, sourceUID, rootUID string, depth int) []string {
	type Node struct {
		UID  string
		Path []string
	}

	visited := make(map[string]bool)
	queue := []Node{{UID: sourceUID, Path: []string{sourceUID}}}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if node.UID == rootUID {
			return node.Path
		}

		visited[node.UID] = true
		currentComponent := components[node.UID]

		for _, dep := range currentComponent.DependsOn {
			if !visited[dep.Uid] {
				queue = append(queue, Node{UID: dep.Uid, Path: append(node.Path, dep.Uid)})
			}
		}

		for _, dep := range currentComponent.ReverseDependsOn {
			if !visited[dep.Uid] {
				queue = append(queue, Node{UID: dep.Uid, Path: append(node.Path, dep.Uid)})
			}
		}

		if len(queue) == 0 && depth > 1 {
			newComponents := fetchDeeperDependencies(node.UID)
			for _, newComponent := range newComponents {
				components[newComponent.Uid] = newComponent
				queue = append(queue, Node{UID: newComponent.Uid, Path: append(node.Path, newComponent.Uid)})
				visited[newComponent.Uid] = true
			}
		}
	}

	return nil
}

func fetchDeeperDependencies(uid string) []Component {
	client := DgraphClient()
	ctx := context.Background()
	query := `
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
			cveID
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
			cveID
			version
			vulnerable
			bomRef
			reference
			root
			}
		}
	}
	`

	query = fmt.Sprintf(query, uid)

	resp, err := client.NewTxn().Query(ctx, query)
	if err != nil {
		log.Printf("Failed to fetch deeper dependencies for UID: %s", uid)
		return nil
	}

	var result QueryResult
	if err := json.Unmarshal(resp.GetJson(), &result); err != nil {
		log.Printf("Failed to unmarshal deeper dependencies for UID: %s", uid)
		return nil
	}

	return result.Component
}
