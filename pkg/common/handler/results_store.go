package handler

import (
	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
	"github.com/timoniersystems/lookout/pkg/common/nvd"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
	"sync"
	"time"
)

type SBOMResults struct {
	CVEPURLPairs     []nvd.CVEPURLPair
	ResultMap        map[string]dgraph.Component
	Components       []cyclonedx.Component // All components from the parsed SBOM
	SeverityFilters  []string
	TotalVulns       int // Total vulnerabilities found before filtering
	FilteredVulns    int // Vulnerabilities after filtering
	Timestamp        time.Time
}

var (
	resultsStore = make(map[string]*SBOMResults)
	resultsMu    sync.RWMutex
)

func StoreResults(sessionID string, cvePairs []nvd.CVEPURLPair, resultMap map[string]dgraph.Component, severityFilters []string, totalCount int, components []cyclonedx.Component) {
	resultsMu.Lock()
	defer resultsMu.Unlock()

	resultsStore[sessionID] = &SBOMResults{
		CVEPURLPairs:    cvePairs,
		ResultMap:       resultMap,
		Components:      components,
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
