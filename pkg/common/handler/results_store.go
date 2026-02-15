package handler

import (
	"timonier.systems/lookout/pkg/common/nvd"
	"timonier.systems/lookout/pkg/ui/dgraph"
	"sync"
	"time"
)

type SBOMResults struct {
	CVEPURLPairs     []nvd.CVEPURLPair
	ResultMap        map[string]dgraph.Component
	SeverityFilters  []string
	TotalVulns       int // Total vulnerabilities found before filtering
	FilteredVulns    int // Vulnerabilities after filtering
	Timestamp        time.Time
}

var (
	resultsStore = make(map[string]*SBOMResults)
	resultsMu    sync.RWMutex
)

func StoreResults(sessionID string, cvePairs []nvd.CVEPURLPair, resultMap map[string]dgraph.Component, severityFilters []string, totalCount int) {
	resultsMu.Lock()
	defer resultsMu.Unlock()

	resultsStore[sessionID] = &SBOMResults{
		CVEPURLPairs:    cvePairs,
		ResultMap:       resultMap,
		SeverityFilters: severityFilters,
		TotalVulns:      totalCount,
		FilteredVulns:   len(cvePairs),
		Timestamp:       time.Now(),
	}

	// Auto-cleanup after 1 hour
	go func(id string) {
		time.Sleep(1 * time.Hour)
		resultsMu.Lock()
		delete(resultsStore, id)
		resultsMu.Unlock()
	}(sessionID)
}

func GetResults(sessionID string) *SBOMResults {
	resultsMu.RLock()
	defer resultsMu.RUnlock()
	return resultsStore[sessionID]
}
