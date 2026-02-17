package dgraph

import (
	"context"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"github.com/timoniersystems/lookout/pkg/graph"
)

// DgraphClientWrapper wraps the Dgraph client to implement graph.DgraphClient
type DgraphClientWrapper struct {
	client *dgo.Dgraph
}

// NewDgraphClientWrapper creates a new wrapper
func NewDgraphClientWrapper() *DgraphClientWrapper {
	return &DgraphClientWrapper{
		client: DgraphClient(),
	}
}

// NewTxn creates a new transaction
func (w *DgraphClientWrapper) NewTxn() graph.DgraphTxn {
	return &DgraphTxnWrapper{txn: w.client.NewTxn()}
}

// DgraphTxnWrapper wraps a Dgraph transaction
type DgraphTxnWrapper struct {
	txn *dgo.Txn
}

// Query executes a query
func (t *DgraphTxnWrapper) Query(ctx context.Context, query string) (graph.DgraphResponse, error) {
	resp, err := t.txn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return &DgraphResponseWrapper{resp: resp}, nil
}

// QueryWithVars executes a query with variables
func (t *DgraphTxnWrapper) QueryWithVars(ctx context.Context, query string, vars map[string]string) (graph.DgraphResponse, error) {
	resp, err := t.txn.QueryWithVars(ctx, query, vars)
	if err != nil {
		return nil, err
	}
	return &DgraphResponseWrapper{resp: resp}, nil
}

// Discard discards the transaction
func (t *DgraphTxnWrapper) Discard(ctx context.Context) {
	_ = t.txn.Discard(ctx)
}

// DgraphResponseWrapper wraps a Dgraph response
type DgraphResponseWrapper struct {
	resp *api.Response
}

// GetJson returns the JSON response
func (r *DgraphResponseWrapper) GetJson() []byte {
	return r.resp.GetJson()
}
