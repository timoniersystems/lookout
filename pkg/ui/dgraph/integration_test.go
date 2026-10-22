//go:build integration

package dgraph_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"

	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
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

// realisticRefs are purl shapes seen in real SBOMs: qualifiers with `&` and
// `=`, a percent-encoded `+` and `@`, and a non-ASCII name.
var realisticRefs = []string{
	"pkg:maven/org.x/a@1.0?type=jar&classifier=sources",
	"pkg:golang/github.com/distribution/distribution@v2.8.2%2Bincompatible",
	"pkg:npm/%40scope/name@1.0.0",
	"pkg:pypi/café@3",
}

// bomWith returns a fresh BOM of n components plus the realistic refs. Fresh on
// every call, because InsertComponentsAndDependencies appends the root to it.
func bomWith(n int) *cyclonedx.Bom {
	bom := &cyclonedx.Bom{
		Metadata: cyclonedx.Metadata{Component: cyclonedx.Component{Name: "img-root", BomRef: "root-ref"}},
	}
	refs := append([]string{}, realisticRefs...)
	for i := 0; i < n; i++ {
		refs = append(refs, fmt.Sprintf("pkg:golang/example.com/c%d@v1.0.%d", i, i))
	}
	for _, ref := range refs {
		bom.Components = append(bom.Components, cyclonedx.Component{Name: ref, Version: "1", Purl: ref, BomRef: ref})
	}
	bom.Dependencies = []cyclonedx.Dependency{{Ref: "root-ref", DependsOn: refs[:3]}}
	return bom
}

func countComponents(t *testing.T, client *dgo.Dgraph) int {
	t.Helper()
	resp, err := client.NewReadOnlyTxn().Query(context.Background(), `{ q(func: has(bomRef)) { count(uid) } }`)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	var out struct {
		Q []struct {
			Count int `json:"count"`
		} `json:"q"`
	}
	if err := json.Unmarshal(resp.Json, &out); err != nil {
		t.Fatalf("count reply: %v", err)
	}
	return out.Q[0].Count
}

func freshGraph(t *testing.T) *dgo.Dgraph {
	t.Helper()
	client := dgraph.DgraphClient()
	if err := dgraph.DropAllData(client); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if err := dgraph.SetupSchema(client); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return client
}

// lookout#63: an insert large enough to need three lookup batches stores every
// component once, and inserting the same BOM again reuses every uid. A lookup
// that silently matched nothing (the list-in-a-variable form does, on v25.4.1)
// would pass the first half and double the count in the second.
func TestInsertReusesExistingComponentsAcrossBatches_Integration(t *testing.T) {
	client := freshGraph(t)
	const n = 1201
	want := n + len(realisticRefs) + 1 // + the root

	digest := "sha256:" + strings.Repeat("a", 64)
	if err := dgraph.InsertComponentsAndDependencies(client, bomWith(n), "img", digest); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if got := countComponents(t, client); got != want {
		t.Fatalf("after the first insert: %d components, want %d", got, want)
	}

	if err := dgraph.InsertComponentsAndDependencies(client, bomWith(n), "img", digest); err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if got := countComponents(t, client); got != want {
		t.Fatalf("re-inserting the same BOM changed the count to %d, want %d: existing uids were not reused", got, want)
	}
}

// The lookup returns exactly the refs that exist: refs a JSON literal has to
// escape (quote, backslash, `&` as &) included, an absent ref excluded.
func TestRetrieveExistingComponentsReturnsOnlyRequestedRefs_Integration(t *testing.T) {
	client := freshGraph(t)
	stored := []string{`pkg:npm/q"uote@1`, `pkg:x/back\slash@2`, realisticRefs[0], realisticRefs[3], "pkg:golang/other@v1"}

	var nodes []map[string]string
	for _, ref := range stored {
		nodes = append(nodes, map[string]string{"bomRef": ref, "dgraph.type": "Component"})
	}
	setJSON, _ := json.Marshal(nodes)
	if _, err := client.NewTxn().Mutate(context.Background(), &api.Mutation{SetJson: setJSON, CommitNow: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	asked := []string{stored[0], stored[1], stored[2], stored[3], "pkg:golang/absent@v9"}
	got, err := dgraph.RetrieveExistingComponents(client.NewReadOnlyTxn(), asked)
	if err != nil {
		t.Fatalf("RetrieveExistingComponents: %v", err)
	}
	for _, ref := range asked[:4] {
		if got[ref] == "" {
			t.Errorf("existing ref %q was not found", ref)
		}
	}
	if _, ok := got["pkg:golang/absent@v9"]; ok {
		t.Error("an absent ref came back")
	}
	if _, ok := got["pkg:golang/other@v1"]; ok {
		t.Error("a ref that was not asked for came back: the lookup is not bounded")
	}
	if len(got) != 4 {
		t.Errorf("got %d refs, want 4: %v", len(got), got)
	}
}
