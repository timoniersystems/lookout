package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
	"github.com/timoniersystems/lookout/pkg/common/nvd"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
)

// TestSBOMResultsJSONRoundTrip guards the serialization contract the Dgraph
// `sessionData` blob relies on (lookout#53): the full SBOMResults payload must
// survive marshal -> store-as-string -> load -> unmarshal without loss, so a GET
// served by a different replica (or after a restart) renders the same page as the
// pod that ran the upload.
func TestSBOMResultsJSONRoundTrip(t *testing.T) {
	orig := &SBOMResults{
		CVEPURLPairs: []nvd.CVEPURLPair{{PURL: "pkg:golang/example@v1.0.0"}},
		ResultMap: map[string]dgraph.Component{
			"pkg:golang/example@v1.0.0": {Name: "example", Version: "v1.0.0"},
		},
		Components: []cyclonedx.Component{
			{Name: "example", Version: "v1.0.0", Purl: "pkg:golang/example@v1.0.0", BomRef: "ref-1"},
		},
		SeverityFilters: []string{"CRITICAL", "HIGH"},
		TotalVulns:      7,
		FilteredVulns:   3,
		Timestamp:       time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC),
	}

	// marshal -> string: exactly what StoreResults writes into sessionData.
	blob, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// unmarshal: exactly what GetResults reads back out of sessionData.
	var got SBOMResults
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.TotalVulns != orig.TotalVulns || got.FilteredVulns != orig.FilteredVulns {
		t.Errorf("vuln counts changed: got %d/%d want %d/%d",
			got.TotalVulns, got.FilteredVulns, orig.TotalVulns, orig.FilteredVulns)
	}
	if !got.Timestamp.Equal(orig.Timestamp) {
		t.Errorf("timestamp changed: got %v want %v", got.Timestamp, orig.Timestamp)
	}
	if len(got.Components) != 1 || got.Components[0].Purl != "pkg:golang/example@v1.0.0" {
		t.Errorf("components not preserved: %+v", got.Components)
	}
	if len(got.ResultMap) != 1 {
		t.Errorf("result map not preserved: %+v", got.ResultMap)
	}
	if len(got.SeverityFilters) != 2 {
		t.Errorf("severity filters not preserved: %+v", got.SeverityFilters)
	}

	// Lossless: re-marshaling the round-tripped value yields identical bytes.
	reblob, err := json.Marshal(&got)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(reblob) != string(blob) {
		t.Errorf("round-trip not byte-identical:\n first: %s\nsecond: %s", blob, reblob)
	}
}
