package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
	"github.com/timoniersystems/lookout/pkg/common/fileutil"
	"github.com/timoniersystems/lookout/pkg/common/nvd"
	"github.com/timoniersystems/lookout/pkg/common/processor"
	"github.com/timoniersystems/lookout/pkg/common/progress"
	"github.com/timoniersystems/lookout/pkg/common/spdx"
	"github.com/timoniersystems/lookout/pkg/common/trivy"
	"github.com/timoniersystems/lookout/pkg/logging"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
	"github.com/timoniersystems/lookout/pkg/validation"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// UploadBOMWithProgress handles SBOM upload with real-time progress updates
func UploadBOMWithProgress(c echo.Context) error {
	// Generate unique session ID
	sessionID := uuid.New().String()

	// Parse multipart form to populate c.Request().Form (required for file uploads)
	if err := c.Request().ParseMultipartForm(32 << 20); err != nil { // 32MB max
		logging.Error("[Session %s] Failed to parse multipart form: %v", sessionID, err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to parse form data",
		})
	}

	// Get severity filters from form
	severityFilters := c.Request().Form["severity"]
	if len(severityFilters) == 0 {
		// Default to HIGH and CRITICAL if none selected
		severityFilters = []string{"CRITICAL", "HIGH"}
	}
	logging.Info("[Session %s] Severity filters: %v", sessionID, severityFilters)

	// Use fileutil to handle temporary file creation
	tempFileHandle, err := fileutil.CreateTempFromFormFile(c, "cyclonedx-bom-file")
	if err != nil {
		logging.Error("[Session %s] Failed to create temporary file: %v", sessionID, err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the BOM file",
		})
	}
	tempFilePath := tempFileHandle.Path
	// Note: We DON'T defer cleanup here because processSBOMWithProgress will handle it

	// Detect BOM format (CycloneDX or SPDX) and validate
	bomFormat, err := validation.DetectBOMFormat(tempFilePath)
	if err != nil {
		logging.Warn("[Session %s] SBOM validation failed: %v", sessionID, err)
		_ = os.Remove(tempFilePath)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}

	switch bomFormat {
	case "cyclonedx":
		if err := validation.ValidateCycloneDXBOM(tempFilePath); err != nil {
			logging.Warn("[Session %s] SBOM validation failed: %v", sessionID, err)
			_ = os.Remove(tempFilePath)
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
			})
		}
	case "spdx":
		if err := validation.ValidateSPDXBOM(tempFilePath); err != nil {
			logging.Warn("[Session %s] SBOM validation failed: %v", sessionID, err)
			_ = os.Remove(tempFilePath)
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Check for obfuscated or suspicious content
	if err := validation.DetectObfuscatedContent(tempFilePath); err != nil {
		logging.Warn("[Session %s] Upload rejected (obfuscated content): %v", sessionID, err)
		_ = os.Remove(tempFilePath)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Create tracker BEFORE rendering page to avoid race condition
	tracker := progress.NewTracker(sessionID)
	tracker.SendProgress("upload", progress.StatusComplete, "SBOM file uploaded successfully", 10)

	// Start async processing with the temp file path, severity filters, and detected format
	go processSBOMWithProgress(sessionID, tempFilePath, severityFilters, tracker, bomFormat)

	// Render progress page AFTER tracker is created and first update sent
	if err := c.Render(http.StatusOK, "progress.html", map[string]interface{}{
		"SessionID": sessionID,
	}); err != nil {
		logging.Info("Failed to render progress page: %v", err)
		_ = os.Remove(tempFilePath)
		tracker.Close()
		return err
	}

	return nil
}

func processSBOMWithProgress(sessionID string, tempFilePath string, severityFilters []string, tracker *progress.Tracker, bomFormat string) {
	defer tracker.Close()
	defer func() { _ = os.Remove(tempFilePath) }()

	// Step 1: File already uploaded (already sent before calling this function)

	// Step 2: Parse BOM
	tracker.SendProgress("parse", progress.StatusActive, "Parsing SBOM and extracting components...", 15)

	var bom *cyclonedx.Bom
	var err error
	switch bomFormat {
	case "spdx":
		bom, err = spdx.ParseBOM(tempFilePath)
	default:
		bom, err = cyclonedx.ParseBOM(tempFilePath)
	}
	if err != nil {
		logging.Error("Failed to parse BOM file: %v", err)
		tracker.SendError(fmt.Sprintf("Failed to parse BOM file: %v", err))
		return
	}

	componentCount := 0
	if bom.Components != nil {
		componentCount = len(bom.Components)
	}
	tracker.SendProgress("parse", progress.StatusComplete, fmt.Sprintf("Parsed %d components", componentCount), 20)

	// Step 3: Build dependency graph (non-fatal - CVE results still shown if Dgraph is unavailable)
	dgraphAvailable := true
	tracker.SendProgress("db", progress.StatusActive, "Building dependency graph...", 25)

	client := dgraph.DgraphClient()
	if err := dgraph.DropAllData(client); err != nil {
		logging.Warn("Failed to drop existing data (Dgraph may be unavailable): %v", err)
		dgraphAvailable = false
	}

	if dgraphAvailable {
		if err := dgraph.SetupSchema(client); err != nil {
			logging.Warn("Failed to setup schema: %v", err)
			dgraphAvailable = false
		}
	}

	if dgraphAvailable {
		if err := dgraph.InsertComponentsAndDependencies(dgraph.DgraphClient(), bom); err != nil {
			logging.Warn("Failed to insert BOM data into Dgraph: %v", err)
			dgraphAvailable = false
		}
	}

	if dgraphAvailable {
		tracker.SendProgress("db", progress.StatusComplete, "Dependency graph built successfully", 45)
	} else {
		tracker.SendProgress("db", progress.StatusComplete, "Dependency graph unavailable - continuing with vulnerability scan", 45)
	}

	// Step 4: Run Trivy scan
	tracker.SendProgress("scan", progress.StatusActive, "Running Trivy vulnerability scanner...", 50)

	trivyResults, err := trivy.RunTrivy(tempFilePath)
	if err != nil {
		logging.Error("Failed to run Trivy on %s: %v", tempFilePath, err)
		tracker.SendError("Failed to run Trivy scanner")
		return
	}

	cvePurlMap, err := processor.ProcessFileInput(trivyResults)
	if err != nil {
		logging.Error("Failed to process Trivy results: %v", err)
		tracker.SendError("Failed to process scan results")
		return
	}

	vulnCount := len(cvePurlMap)
	tracker.SendProgress("scan", progress.StatusComplete, fmt.Sprintf("Found %d potential vulnerabilities", vulnCount), 60)

	// Step 5: Fetch CVE data
	tracker.SendProgress("cve", progress.StatusActive, fmt.Sprintf("Fetching CVE data for %d vulnerabilities...", vulnCount), 65)

	// Update Dgraph with CVE info (non-fatal)
	if dgraphAvailable {
		_ = dgraph.QueryAndUpdatePurl(cvePurlMap)
	}

	// Fetch CVE data from NVD
	tracker.SendProgress("cve", progress.StatusActive, "Retrieving vulnerability details from NVD (may be slow due to rate limits)...", 70)
	aggregatedData := nvd.AggregateCVEData(cvePurlMap)

	tracker.SendProgress("cve", progress.StatusComplete, fmt.Sprintf("Retrieved data for %d CVEs", len(aggregatedData)), 75)

	// Step 6: Trace dependency paths (non-fatal)
	resultMap := make(map[string]dgraph.Component)
	if dgraphAvailable {
		tracker.SendProgress("paths", progress.StatusActive, "Tracing dependency paths to vulnerable packages...", 78)

		logging.Info("[Session %s] Starting RetrieveVulnerablePURLs for %d CVEs", sessionID, len(cvePurlMap))
		var pathErr error
		resultMap, pathErr = dgraph.RetrieveVulnerablePURLs(cvePurlMap)
		if pathErr != nil {
			logging.Warn("[Session %s] Failed to retrieve vulnerable PURLs: %v", sessionID, pathErr)
			resultMap = make(map[string]dgraph.Component)
		} else {
			logging.Info("[Session %s] RetrieveVulnerablePURLs completed, got %d results", sessionID, len(resultMap))
		}

		tracker.SendProgress("paths", progress.StatusComplete, "Dependency paths traced", 82)
	} else {
		tracker.SendProgress("paths", progress.StatusComplete, "Dependency path tracing skipped (Dgraph unavailable)", 82)
	}

	// Step 7: Filter by severity
	tracker.SendProgress("filter", progress.StatusActive, "Filtering results by severity...", 85)

	// Build page data with severity filtering
	totalCount := len(aggregatedData)
	pageData := buildFilteredResultsPageData(aggregatedData, resultMap, severityFilters)

	filteredCount := len(pageData.CVEPURLPairs)
	logging.Info("[Session %s] Filtered %d/%d vulnerabilities matching severity filter %v", sessionID, filteredCount, totalCount, severityFilters)
	tracker.SendProgress("filter", progress.StatusComplete, fmt.Sprintf("Filtered to %d/%d vulnerabilities matching selected severity levels", filteredCount, totalCount), 90)

	// Step 8: Finalize results
	tracker.SendProgress("complete", progress.StatusActive, "Preparing final report...", 95)

	// Store results for retrieval with severity filter information
	StoreResults(sessionID, pageData.CVEPURLPairs, resultMap, severityFilters, totalCount, bom.Components)

	tracker.SendProgress("complete", progress.StatusComplete, "Analysis complete!", 100)

	// Send completion with redirect
	tracker.SendComplete("/results/" + sessionID)
}

// buildFilteredResultsPageData builds results with severity filtering
func buildFilteredResultsPageData(aggregatedData []nvd.CVEPURLPair, resultMap map[string]dgraph.Component, severityFilters []string) nvd.ResultsPageData {
	var pageData nvd.ResultsPageData

	for _, data := range aggregatedData {
		// Filter by severity first
		if !filterBySeverity(data, severityFilters) {
			continue
		}

		// Match CVE with dependency data from Dgraph
		purl := strings.TrimSpace(strings.ToLower(data.PURL))
		for _, component := range resultMap {
			componentPurl := strings.TrimSpace(strings.ToLower(component.Purl))
			if componentPurl == purl {
				data.DgraphData = component
				break
			}
		}
		pageData.CVEPURLPairs = append(pageData.CVEPURLPairs, data)
	}

	// Sort results: first by severity (CRITICAL → HIGH → MEDIUM → LOW → N/A),
	// then by CVSS score descending within each severity level
	sortCVEPURLPairs(pageData.CVEPURLPairs)

	return pageData
}
