// Package repository provides data access layer abstraction for Dgraph operations.
package repository

import (
	"context"
	"fmt"
	"log"

	"lookout/pkg/common/cyclonedx"
	"lookout/pkg/ui/dgraph"

	"github.com/dgraph-io/dgo/v210"
)

// DgraphRepository provides methods for interacting with the Dgraph database.
type DgraphRepository struct {
	clientManager *dgraph.DgraphClientManager
}

// NewDgraphRepository creates a new Dgraph repository with the given client manager.
func NewDgraphRepository(clientManager *dgraph.DgraphClientManager) *DgraphRepository {
	return &DgraphRepository{
		clientManager: clientManager,
	}
}

// getClient is a helper method to get the Dgraph client.
func (r *DgraphRepository) getClient() (*dgo.Dgraph, error) {
	return r.clientManager.GetClient()
}

// InsertComponents inserts BOM components and their dependencies into Dgraph.
func (r *DgraphRepository) InsertComponents(ctx context.Context, bom *cyclonedx.Bom) error {
	client, err := r.getClient()
	if err != nil {
		return fmt.Errorf("failed to get Dgraph client: %w", err)
	}

	return dgraph.InsertComponentsAndDependencies(client, bom)
}

// UpdateVulnerabilities updates vulnerability information for components.
func (r *DgraphRepository) UpdateVulnerabilities(ctx context.Context, cvePurlMap map[string]string) error {
	return dgraph.QueryAndUpdatePurl(cvePurlMap)
}

// RetrieveVulnerablePURLs retrieves vulnerable components for given CVE/PURL pairs.
func (r *DgraphRepository) RetrieveVulnerablePURLs(ctx context.Context, cvePurlMap map[string]string) (map[string]dgraph.Component, error) {
	return dgraph.RetrieveVulnerablePURLs(cvePurlMap)
}

// RetrievePURL retrieves component information for a specific PURL.
func (r *DgraphRepository) RetrievePURL(ctx context.Context, purl string) (dgraph.FilteredResult, error) {
	return dgraph.RetrievePURL(purl)
}

// WaitForDataIndexed is a no-op now that we use Dgraph's recursive query directly.
// Previously this was used to wait for async indexing, but the new approach of
// extracting paths directly from Dgraph's @recurse query result is synchronous.
func (r *DgraphRepository) WaitForDataIndexed(ctx context.Context, purl string) error {
	// No-op: Dgraph's recursive query is synchronous and returns indexed data
	return nil
}

// DropAllData drops all data from the database while preserving the schema.
func (r *DgraphRepository) DropAllData(ctx context.Context) error {
	client, err := r.getClient()
	if err != nil {
		return fmt.Errorf("failed to get Dgraph client: %w", err)
	}

	return dgraph.DropAllData(client)
}

// SetupSchema initializes the Dgraph schema.
func (r *DgraphRepository) SetupSchema(ctx context.Context) error {
	// Schema setup is handled by SetupAndRunDgraph()
	// This method is provided for interface compatibility
	log.Println("Schema setup should be called via dgraph.SetupAndRunDgraph()")
	return nil
}

// Close closes the repository's database connection.
func (r *DgraphRepository) Close() error {
	return r.clientManager.Close()
}
