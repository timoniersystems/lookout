package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"lookout/pkg/logging"
)

// QueryBuilder builds Dgraph queries
type QueryBuilder struct {
	client DgraphClient
	depth  int // How many levels to nest dependencies
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(depth int, client DgraphClient) *QueryBuilder {
	return &QueryBuilder{
		client: client,
		depth:  depth,
	}
}

// QueryByPURL queries for a component by PURL
func (q *QueryBuilder) QueryByPURL(ctx context.Context, purl string) ([]Component, error) {
	query := fmt.Sprintf(`
	query PurlQuery($purl: string) {
		component(func: eq(purl, $purl)) {
			%s
		}
	}
	`, q.componentFields(q.depth))

	variables := map[string]string{"$purl": purl}

	txn := q.client.NewTxn()
	defer txn.Discard(ctx)

	resp, err := txn.QueryWithVars(ctx, query, variables)
	if err != nil {
		logging.Error("Query failed for PURL: %s", purl)
		return nil, err
	}

	var result struct {
		Component []Component `json:"component"`
	}
	if err := json.Unmarshal(resp.GetJson(), &result); err != nil {
		logging.Error("Failed to unmarshal response for PURL: %s", purl)
		return nil, err
	}

	return result.Component, nil
}

// QueryByPURLAndCVE queries for a component by PURL and CVE ID
func (q *QueryBuilder) QueryByPURLAndCVE(ctx context.Context, purl, cveID string) ([]Component, error) {
	query := fmt.Sprintf(`
	query PurlAndCveQuery($purl: string, $cveID: string) {
		component(func: eq(purl, $purl)) @filter(eq(cveID, $cveID)) {
			%s
		}
	}
	`, q.componentFields(q.depth))

	variables := map[string]string{"$purl": purl, "$cveID": cveID}

	txn := q.client.NewTxn()
	defer txn.Discard(ctx)

	resp, err := txn.QueryWithVars(ctx, query, variables)
	if err != nil {
		return nil, fmt.Errorf("query failed for CVE %s and PURL %s: %w", cveID, purl, err)
	}

	var result struct {
		Component []Component `json:"component"`
	}
	if err := json.Unmarshal(resp.GetJson(), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Component, nil
}

// QueryRootComponents queries for all root components
func (q *QueryBuilder) QueryRootComponents(ctx context.Context) ([]Component, error) {
	query := fmt.Sprintf(`
	{
		component(func: eq(root, true)) {
			%s
		}
	}
	`, q.componentFields(q.depth))

	txn := q.client.NewTxn()
	defer txn.Discard(ctx)

	resp, err := txn.Query(ctx, query)
	if err != nil {
		logging.Error("Query failed for root component")
		return nil, err
	}

	var result struct {
		Component []Component `json:"component"`
	}
	if err := json.Unmarshal(resp.GetJson(), &result); err != nil {
		logging.Error("Failed to unmarshal response for root component")
		return nil, err
	}

	return result.Component, nil
}

// QueryPathToRoot uses recursive query to find path from component to root
func (q *QueryBuilder) QueryPathToRoot(ctx context.Context, purl string) ([]Component, error) {
	// This query recursively follows reverse dependencies (~dependsOn) from the component to root
	query := `
	query PathToRoot($purl: string) {
		component(func: eq(purl, $purl)) @recurse(depth: 10, loop: false) {
			uid
			name
			purl
			version
			vulnerable
			bomRef
			reference
			root
			~dependsOn
		}
	}
	`

	variables := map[string]string{"$purl": purl}

	txn := q.client.NewTxn()
	defer txn.Discard(ctx)

	resp, err := txn.QueryWithVars(ctx, query, variables)
	if err != nil {
		logging.Error("Recursive query failed for PURL: %s", purl)
		return nil, err
	}

	var result struct {
		Component []Component `json:"component"`
	}
	if err := json.Unmarshal(resp.GetJson(), &result); err != nil {
		logging.Error("Failed to unmarshal recursive query response for PURL: %s", purl)
		return nil, err
	}

	return result.Component, nil
}

// componentFields generates nested component fields for given depth
func (q *QueryBuilder) componentFields(depth int) string {
	baseFields := `uid
			name
			purl
			cveID
			version
			vulnerable
			bomRef
			reference
			root`

	if depth <= 0 {
		return baseFields
	}

	depFields := q.dependencyFields(depth - 1)
	return fmt.Sprintf(`%s
			dependsOn {
				%s
			}
			~dependsOn {
				%s
			}`, baseFields, depFields, depFields)
}

// dependencyFields generates nested dependency fields
func (q *QueryBuilder) dependencyFields(depth int) string {
	baseFields := `uid
				name
				purl
				version
				vulnerable
				bomRef
				reference
				root`

	if depth <= 0 {
		return baseFields
	}

	subDepFields := q.dependencyFields(depth - 1)
	return fmt.Sprintf(`%s
				dependsOn {
					%s
				}
				~dependsOn {
					%s
				}`, baseFields, subDepFields, subDepFields)
}
