package graph

import "context"

// DgraphClient represents a Dgraph database client
type DgraphClient interface {
	NewTxn() DgraphTxn
}

// DgraphTxn represents a Dgraph transaction
type DgraphTxn interface {
	Query(ctx context.Context, query string) (DgraphResponse, error)
	QueryWithVars(ctx context.Context, query string, vars map[string]string) (DgraphResponse, error)
	Discard(ctx context.Context)
}

// DgraphResponse represents a Dgraph query response
type DgraphResponse interface {
	GetJson() []byte
}
