package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type FilteredResult struct {
	SearchedPurl      string   `json:"searched_purl"`
	PathToRootPackage []string `json:"path_to_root_package,omitempty"`
}

func leadsToRoot(purl string, resultMap map[string]Component) bool {
	for _, component := range resultMap {
		if component.Purl == purl && component.Root {
			return true
		}
		for _, dep := range component.DependsOn {
			if dep.Purl == purl && component.Root {
				return true
			}
		}
	}
	return false
}

func RetrievePURL(pURL string) (FilteredResult, error) {
	client := DgraphClient()
	ctx := context.Background()

	query := `
	query PurlQuery($purl: string) {
		component(func: eq(purl, $purl)) {
			uid
			name
			purl
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
	`

	resultMap := make(map[string]Component)

	log.Printf("Querying for PURL: %s", pURL)
	variables := map[string]string{"$purl": pURL}

	txn := client.NewTxn()
	defer txn.Discard(ctx)

	resp, err := txn.QueryWithVars(context.Background(), query, variables)
	if err != nil {
		log.Printf("Query failed for PURL: %s", pURL)
		return FilteredResult{}, err

	}

	var result QueryResult
	if err := json.Unmarshal(resp.GetJson(), &result); err != nil {
		log.Printf("Failed to unmarshal response for PURL: %s", pURL)
		return FilteredResult{}, err

	}

	for _, component := range result.Component {
		if len(component.DependsOn) == 0 {
			component.DependsOn = []DependsOn{{Name: "No dependencies found"}}
		}
		if len(component.ReverseDependsOn) == 0 {
			component.ReverseDependsOn = []DependsOn{{Name: "No dependencies found"}}
		}
		resultMap[component.Uid] = component
		log.Printf("Component found: UID: %s, Name: %s, PURL: %s", component.Uid, component.Name, component.Purl)
	}

	rootQuery := `
	{
		component(func: eq(root, true)) {
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
	`

	rootTxn := client.NewTxn()
	defer rootTxn.Discard(ctx)

	rootResp, err := rootTxn.Query(context.Background(), rootQuery)
	if err != nil {
		log.Printf("Query failed for root component")
		return FilteredResult{}, err

	}

	var rootResult QueryResult
	if err := json.Unmarshal(rootResp.GetJson(), &rootResult); err != nil {
		log.Printf("Failed to unmarshal response for root component")
		return FilteredResult{}, err
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
		return FilteredResult{}, nil
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
	var filteredComponent *Component
	for _, component := range resultMap {
		if component.Purl == pURL {
			filteredComponent = &component
			break
		}
	}

	if filteredComponent == nil {
		log.Printf("No component found for searched pURL: %s", pURL)
		return FilteredResult{}, fmt.Errorf("no component found for pURL: %s", pURL)
	}

	dependsOnPurls := []string{}
	for _, dep := range filteredComponent.DependsOn {
		dependsOnPurls = append(dependsOnPurls, dep.Purl)
	}

	reverseDependsOnPurls := []string{}
	for _, revDep := range filteredComponent.ReverseDependsOn {
		reverseDependsOnPurls = append(reverseDependsOnPurls, revDep.Purl)
	}

	dependsOnReachesRoot := false
	reverseDependsOnReachesRoot := false

	for _, purl := range dependsOnPurls {
		if leadsToRoot(purl, resultMap) {
			dependsOnReachesRoot = true
			break
		}
	}

	for _, purl := range reverseDependsOnPurls {
		if leadsToRoot(purl, resultMap) {
			reverseDependsOnReachesRoot = true
			break
		}
	}

	filteredResult := FilteredResult{
		SearchedPurl: filteredComponent.Purl,
	}

	if dependsOnReachesRoot {
		filteredResult.PathToRootPackage = dependsOnPurls
	} else if reverseDependsOnReachesRoot {
		filteredResult.PathToRootPackage = reverseDependsOnPurls
	}

	log.Printf("Returning filtered data for pURL: %s", pURL)
	return filteredResult, nil

}

func fetchDeeperPurlDependencies(uid string) []Component {
	client := DgraphClient()
	ctx := context.Background()
	query := `
	{
		component(func: uid(%s)) {
			uid
			name
			purl
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
