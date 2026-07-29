package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/dgo/v210/protos/api"
	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
	"github.com/timoniersystems/lookout/pkg/common/nvd"
	"github.com/timoniersystems/lookout/pkg/logging"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
)

// SBOMResults is the per-session SBOM analysis payload rendered by
// GET /results/:sessionID.
type SBOMResults struct {
	CVEPURLPairs    []nvd.CVEPURLPair
	ResultMap       map[string]dgraph.Component
	Components      []cyclonedx.Component // All components from the parsed SBOM
	SeverityFilters []string
	TotalVulns      int // Total vulnerabilities found before filtering
	FilteredVulns   int // Vulnerabilities after filtering
	Timestamp       time.Time
}

// sessionTTL bounds how long a stored result is served before it is treated as
// expired (read-time) and reaped (sweep).
const sessionTTL = 1 * time.Hour

// dgraphTimeout bounds each Dgraph round-trip so a hung Dgraph never wedges an
// HTTP handler.
const dgraphTimeout = 15 * time.Second

// sessionNode is the Dgraph representation of a stored session. The full
// SBOMResults payload is JSON-serialized into the scalar `sessionData` predicate
// rather than modeled as graph nodes — these results are an opaque blob keyed by
// session ID, not part of the dependency graph. `sessionExpiresAt` is an RFC3339
// UTC string (fixed-width, so lexical order == chronological) so the sweep's
// `lt()` inequality works on the exact string index.
type sessionNode struct {
	Type      string `json:"dgraph.type,omitempty"`
	SessionID string `json:"sessionID,omitempty"`
	Data      string `json:"sessionData,omitempty"`
	ExpiresAt string `json:"sessionExpiresAt,omitempty"`
}

// StoreResults persists a session's SBOM analysis results to Dgraph, keyed by
// session ID, with a TTL (lookout#53). Persisting — rather than the old
// per-process in-memory map — means ANY replica can serve
// GET /results/:sessionID (replicaCount > 1) and a restart does not lose active
// sessions.
//
// Best-effort: a Dgraph error is logged, not returned, to preserve the previous
// void signature/behavior (the caller does not check a return, and a failed
// store simply surfaces later as a "not found or expired" GET — the same
// user-visible outcome as the old in-memory map after a TTL/restart).
func StoreResults(sessionID string, cvePairs []nvd.CVEPURLPair, resultMap map[string]dgraph.Component, severityFilters []string, totalCount int, components []cyclonedx.Component) {
	results := &SBOMResults{
		CVEPURLPairs:    cvePairs,
		ResultMap:       resultMap,
		Components:      components,
		SeverityFilters: severityFilters,
		TotalVulns:      totalCount,
		FilteredVulns:   len(cvePairs),
		Timestamp:       time.Now(),
	}

	payload, err := json.Marshal(results)
	if err != nil {
		logging.Error("StoreResults: marshal results for session %s: %v", sessionID, err)
		return
	}

	node := sessionNode{
		Type:      "Session",
		SessionID: sessionID,
		Data:      string(payload),
		ExpiresAt: time.Now().Add(sessionTTL).UTC().Format(time.RFC3339),
	}
	setJSON, err := json.Marshal(node)
	if err != nil {
		logging.Error("StoreResults: marshal node for session %s: %v", sessionID, err)
		return
	}

	client, err := dgraph.GetGlobalClientManager().GetClient()
	if err != nil {
		logging.Error("StoreResults: dgraph client for session %s: %v", sessionID, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), dgraphTimeout)
	defer cancel()

	txn := client.NewTxn()
	defer func() { _ = txn.Discard(ctx) }()
	if _, err := txn.Mutate(ctx, &api.Mutation{SetJson: setJSON, CommitNow: true}); err != nil {
		logging.Error("StoreResults: dgraph mutate for session %s: %v", sessionID, err)
		return
	}

	startSessionSweeper()
}

// GetResults fetches a session's results from Dgraph by session ID, returning nil
// if absent or expired (the handler renders "not found or expired" on nil). A
// session ID is a server-generated UUID, so embedding it in the query with %q is
// safe.
func GetResults(sessionID string) *SBOMResults {
	client, err := dgraph.GetGlobalClientManager().GetClient()
	if err != nil {
		logging.Error("GetResults: dgraph client for session %s: %v", sessionID, err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), dgraphTimeout)
	defer cancel()

	txn := client.NewTxn()
	defer func() { _ = txn.Discard(ctx) }()

	q := fmt.Sprintf(`{ session(func: eq(sessionID, %q), first: 1) { sessionData sessionExpiresAt } }`, sessionID)
	resp, err := txn.Query(ctx, q)
	if err != nil {
		logging.Error("GetResults: dgraph query for session %s: %v", sessionID, err)
		return nil
	}

	var out struct {
		Session []struct {
			Data      string `json:"sessionData"`
			ExpiresAt string `json:"sessionExpiresAt"`
		} `json:"session"`
	}
	if err := json.Unmarshal(resp.GetJson(), &out); err != nil {
		logging.Error("GetResults: unmarshal response for session %s: %v", sessionID, err)
		return nil
	}
	if len(out.Session) == 0 {
		return nil
	}

	rec := out.Session[0]
	// Read-time expiry is the authoritative TTL guard (the sweep is only
	// housekeeping); serve nothing past expiresAt even if the sweep has not run.
	if exp, perr := time.Parse(time.RFC3339, rec.ExpiresAt); perr == nil && time.Now().After(exp) {
		return nil
	}

	var results SBOMResults
	if err := json.Unmarshal([]byte(rec.Data), &results); err != nil {
		logging.Error("GetResults: unmarshal payload for session %s: %v", sessionID, err)
		return nil
	}
	return &results
}

// startSessionSweeper launches, once per process, a periodic reaper of expired
// Session nodes so Dgraph does not accumulate them unbounded. Read-time expiry
// in GetResults is authoritative for correctness; this is housekeeping only, so
// an error is logged and ignored. Idempotent, so it is safe under replicaCount>1
// (each replica sweeps; deleting an already-deleted node is a no-op).
var sweeperOnce sync.Once

func startSessionSweeper() {
	sweeperOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(sessionTTL)
			defer ticker.Stop()
			for range ticker.C {
				sweepExpiredSessions()
			}
		}()
	})
}

func sweepExpiredSessions() {
	client, err := dgraph.GetGlobalClientManager().GetClient()
	if err != nil {
		logging.Error("sweepExpiredSessions: dgraph client: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), dgraphTimeout*2)
	defer cancel()

	now := time.Now().UTC().Format(time.RFC3339)
	txn := client.NewTxn()
	defer func() { _ = txn.Discard(ctx) }()

	req := &api.Request{
		Query:     fmt.Sprintf(`{ expired as var(func: lt(sessionExpiresAt, %q)) @filter(type(Session)) }`, now),
		Mutations: []*api.Mutation{{DelNquads: []byte("uid(expired) * * .")}},
		CommitNow: true,
	}
	if _, err := txn.Do(ctx, req); err != nil {
		logging.Error("sweepExpiredSessions: %v", err)
	}
}
