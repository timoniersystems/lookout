package dgraph_test

import (
	"context"
	"testing"

	"github.com/dgraph-io/dgo/v210/protos/api"

	"defender/pkg/gui/dgraph"
)

func TestRetrieveVulnerablePURLs_Integration(t *testing.T) {
	t.Log("Starting TestRetrieveVulnerablePURLs_Integration")

	client := dgraph.DgraphClient()
	t.Log("Connected to Dgraph client")

	err := dgraph.DropAllData(client)
	if err != nil {
		t.Fatalf("Failed to drop data: %v", err)
	}
	t.Log("Dropped all existing data")

	mutation := `
	{
		set {
			_:c1 <name> "lib-a" .
			_:c1 <version> "1.0.0" .
			_:c1 <purl> "pkg:npm/lib-a@1.0.0" .
			_:c1 <cveID> "CVE-2024-9999" .
			_:c1 <vulnerable> "true" .
			_:c1 <dependsOn> _:c2 .
			_:c1 <dgraph.type> "Component" .

			_:c2 <name> "lib-b" .
			_:c2 <version> "2.0.0" .
			_:c2 <purl> "pkg:npm/lib-b@2.0.0" .
			_:c2 <root> "true" .
			_:c2 <dgraph.type> "Component" .
		}
	}`

	txn := client.NewTxn()
	defer txn.Discard(context.Background())

	t.Log("Starting mutation")
	if _, err := txn.Mutate(context.Background(), &api.Mutation{
		SetNquads: []byte(mutation),
		CommitNow: true,
	}); err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}
	t.Log("Mutation completed")

	purls := map[string]string{
		"CVE-2024-9999": "pkg:npm/lib-a@1.0.0",
	}

	t.Log("Calling RetrieveVulnerablePURLs")
	result, err := dgraph.RetrieveVulnerablePURLs(purls)
	if err != nil {
		t.Fatalf("RetrieveVulnerablePURLs failed: %v", err)
	}
	t.Logf("Received result: %+v", result)

	if len(result) == 0 {
		t.Fatal("Expected vulnerable component in result, got none")
	}

	comp := result[getOnlyKey(result)]
	if len(comp.DependsOn) == 0 || comp.DependsOn[len(comp.DependsOn)-1].Name != "lib-b" {
		t.Errorf("Expected path to root ending in 'lib-b', got %v", comp.DependsOn)
	}
	t.Log("Finished TestRetrieveVulnerablePURLs_Integration")
}

func getOnlyKey(m map[string]dgraph.Component) string {
	for k := range m {
		return k
	}
	return ""
}
