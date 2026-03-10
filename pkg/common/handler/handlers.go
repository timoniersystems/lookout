package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
	"github.com/timoniersystems/lookout/pkg/common/fileutil"
	"github.com/timoniersystems/lookout/pkg/common/nvd"
	"github.com/timoniersystems/lookout/pkg/common/processor"
	"github.com/timoniersystems/lookout/pkg/common/spdx"
	"github.com/timoniersystems/lookout/pkg/common/trivy"
	"github.com/timoniersystems/lookout/pkg/logging"
	"github.com/timoniersystems/lookout/pkg/repository"
	"github.com/timoniersystems/lookout/pkg/service"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
	"github.com/timoniersystems/lookout/pkg/validation"

	"github.com/labstack/echo/v4"
)

// HandlerDependencies holds the dependencies needed by HTTP handlers
type HandlerDependencies struct {
	VulnService *service.VulnerabilityService
	Repo        *repository.DgraphRepository
}

type TemplateData struct {
	CVEs []nvd.CVEPURLPair
}

type ResultsTemplateData struct {
	CVEIDs      []string
	SelectedCVE *nvd.CVEData
}

// filterBySeverity checks if a CVE matches any of the selected severity levels
func filterBySeverity(pair nvd.CVEPURLPair, severityFilters []string) bool {
	if len(severityFilters) == 0 {
		return true // No filter means show all
	}

	// Create a map for quick lookup
	severityMap := make(map[string]bool)
	for _, sev := range severityFilters {
		severityMap[strings.ToUpper(sev)] = true
	}

	// If all four standard severity levels are selected, include CVEs with "N/A" severity as well
	allSeveritiesSelected := severityMap["CRITICAL"] && severityMap["HIGH"] &&
	                         severityMap["MEDIUM"] && severityMap["LOW"]

	// Check if any vulnerability in the pair matches the severity filter
	for _, vuln := range pair.Data.Vulnerabilities {
		if len(vuln.CVE.Metrics.CvssMetricV31) > 0 {
			baseSeverityRaw := vuln.CVE.Metrics.CvssMetricV31[0].CvssData.BaseSeverity
			severity := strings.ToUpper(baseSeverityRaw)

			// Skip if severity is empty
			if baseSeverityRaw == "" {
				continue
			}

			// If severity is "N/A" and user selected all severities, include it
			if severity == "N/A" && allSeveritiesSelected {
				logging.Debug(" CVE %s has N/A severity, including because all severities selected", vuln.CVE.ID)
				return true
			}

			// Check if severity matches the selected filters
			if severityMap[severity] {
				logging.Debug(" CVE %s (severity=%s) MATCHED filter", vuln.CVE.ID, severity)
				return true
			}
		} else if allSeveritiesSelected {
			// No CVSS v3.1 metrics available
			logging.Debug(" CVE %s has no CvssMetricV31 data, including because all severities selected", vuln.CVE.ID)
			return true
		}
	}

	return false
}

// getSeverityPriority returns a numeric priority for sorting (lower is higher priority)
func getSeverityPriority(severity string) int {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return 1
	case "HIGH":
		return 2
	case "MEDIUM":
		return 3
	case "LOW":
		return 4
	default:
		return 5 // N/A or unknown
	}
}

// getCVSSScore extracts the CVSS base score from a CVEPURLPair
func getCVSSScore(pair nvd.CVEPURLPair) float64 {
	if len(pair.Data.Vulnerabilities) > 0 {
		vuln := pair.Data.Vulnerabilities[0]
		if len(vuln.CVE.Metrics.CvssMetricV31) > 0 {
			return vuln.CVE.Metrics.CvssMetricV31[0].CvssData.BaseScore
		}
	}
	return 0.0
}

// getCVESeverity extracts the severity from a CVEPURLPair
func getCVESeverity(pair nvd.CVEPURLPair) string {
	if len(pair.Data.Vulnerabilities) > 0 {
		vuln := pair.Data.Vulnerabilities[0]
		if len(vuln.CVE.Metrics.CvssMetricV31) > 0 {
			severity := vuln.CVE.Metrics.CvssMetricV31[0].CvssData.BaseSeverity
			if severity == "" {
				return "N/A"
			}
			return severity
		}
	}
	return "N/A"
}

// sortCVEPURLPairs sorts CVE pairs by severity and score
func sortCVEPURLPairs(pairs []nvd.CVEPURLPair) {
	sort.Slice(pairs, func(i, j int) bool {
		sevI := getCVESeverity(pairs[i])
		sevJ := getCVESeverity(pairs[j])
		priI := getSeverityPriority(sevI)
		priJ := getSeverityPriority(sevJ)

		// First, sort by severity priority
		if priI != priJ {
			return priI < priJ
		}

		// Within same severity, sort by score descending (higher scores first)
		scoreI := getCVSSScore(pairs[i])
		scoreJ := getCVSSScore(pairs[j])
		return scoreI > scoreJ
	})
}

func CVES(c echo.Context) error {
	cveID := c.FormValue("cveID")

	if cveID != "" {
		data, err := nvd.FetchCVEData(cveID)
		if err != nil {
			logging.Error("Error fetching CVE data for %s: %v", cveID, err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to fetch CVE data",
			})
		}
		return c.Render(http.StatusOK, "cve_details.html", data)
	}

	file, err := c.FormFile("file")
	if err == nil && file != nil {
		// Use fileutil to handle temporary file creation
		tempFileHandle, err := fileutil.CreateTempFromFormFile(c, "file")
		if err != nil {
			logging.Error("Failed to create temporary file: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to process uploaded file",
			})
		}
		defer func() { _ = tempFileHandle.Cleanup() }()

		// Validate file content based on extension
		ext := strings.ToLower(filepath.Ext(tempFileHandle.Path))
		switch ext {
		case ".txt":
			if err := validation.ValidateCVETextFile(tempFileHandle.Path); err != nil {
				logging.Warn("Upload validation failed: %v", err)
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error": err.Error(),
				})
			}
		case ".json":
			if err := validation.ValidateTrivyJSON(tempFileHandle.Path); err != nil {
				logging.Warn("Upload validation failed: %v", err)
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error": err.Error(),
				})
			}
		}

		cveIDs, err := processor.ProcessFileInputForCVEs(tempFileHandle.Path)
		if err != nil {
			logging.Error("Failed to extract CVE IDs from uploaded file: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to extract CVE IDs",
			})
		}

		return c.Render(http.StatusOK, "cve_list.html", map[string]interface{}{
			"CVEs": cveIDs,
		})
	}

	return c.JSON(http.StatusBadRequest, map[string]interface{}{
		"error": "No CVE ID or file provided",
	})
}

func ProcessCVE(c echo.Context) error {
	cveID := c.FormValue("cveID")

	data, err := nvd.FetchCVEData(cveID)
	if err != nil {
		logging.Error("Error fetching CVE data for %s: %v", cveID, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to fetch CVE data",
		})
	}

	return c.Render(http.StatusOK, "results.html", data)
}

func UploadAndProcess(c echo.Context) error {
	// Use fileutil to handle temporary file creation
	tempFileHandle, err := fileutil.CreateTempFromFormFile(c, "file")
	if err != nil {
		logging.Error("Failed to create temporary file: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the file",
		})
	}
	defer func() { _ = tempFileHandle.Cleanup() }()

	logging.Debug("Processing uploaded file at: %s", tempFileHandle.Path)

	// Validate file content based on extension
	ext := strings.ToLower(filepath.Ext(tempFileHandle.Path))
	switch ext {
	case ".txt":
		if err := validation.ValidateCVETextFile(tempFileHandle.Path); err != nil {
			logging.Warn("Upload validation failed: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
			})
		}
	case ".json":
		if err := validation.ValidateTrivyJSON(tempFileHandle.Path); err != nil {
			logging.Warn("Upload validation failed: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	cvePurlMap, err := processor.ProcessFileInput(tempFileHandle.Path)
	if err != nil {
		logging.Error("Failed to process uploaded file at %s: %v", tempFileHandle.Path, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to process uploaded file",
		})
	}

	aggregatedData := nvd.AggregateCVEData(cvePurlMap)

	var pageData nvd.ResultsPageData
	pageData.CVEPURLPairs = append(pageData.CVEPURLPairs, aggregatedData...)

	// Sort results by severity (CRITICAL → HIGH → MEDIUM → LOW → N/A) and score
	sortCVEPURLPairs(pageData.CVEPURLPairs)

	return c.Render(http.StatusOK, "cve_results.html", pageData)
}

func RunTrivyAndProcess(c echo.Context) error {
	if !trivy.CheckTrivyInstalled() {
		logging.Warn("Trivy is not installed")
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Please install Trivy before running the application",
		})
	}

	// Get severity filters from form
	severityFilters := c.Request().Form["severity"]
	if len(severityFilters) == 0 {
		// Default to HIGH and CRITICAL if none selected
		severityFilters = []string{"CRITICAL", "HIGH"}
	}
	logging.Debug("Severity filters: %v", severityFilters)

	// Use fileutil to handle temporary file creation
	tempFileHandle, err := fileutil.CreateTempFromFormFile(c, "sbom-file")
	if err != nil {
		logging.Error("Failed to create temporary file: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the file",
		})
	}
	defer func() { _ = tempFileHandle.Cleanup() }()

	logging.Info("Running Trivy on uploaded file: %s", tempFileHandle.Path)

	trivyResults, err := trivy.RunTrivy(tempFileHandle.Path)
	if err != nil {
		logging.Error("Failed to run Trivy on %s: %v", tempFileHandle.Path, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to run Trivy scanner",
		})
	}

	cvePurlMap, err := processor.ProcessFileInput(trivyResults)
	if err != nil {
		logging.Error("Failed to process Trivy results: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to process uploaded file",
		})
	}

	aggregatedData := nvd.AggregateCVEData(cvePurlMap)

	// Filter by severity
	var pageData nvd.ResultsPageData
	for _, data := range aggregatedData {
		if filterBySeverity(data, severityFilters) {
			pageData.CVEPURLPairs = append(pageData.CVEPURLPairs, data)
		}
	}

	return c.Render(http.StatusOK, "cve_results.html", pageData)
}

// UploadBOMAndInsertData handles BOM file upload and vulnerability analysis
// This function requires HandlerDependencies to be injected
func UploadBOMAndInsertData(deps *HandlerDependencies) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		// Get severity filters from form
		severityFilters := c.Request().Form["severity"]
		if len(severityFilters) == 0 {
			// Default to HIGH and CRITICAL if none selected
			severityFilters = []string{"CRITICAL", "HIGH"}
		}
		logging.Debug("Severity filters: %v", severityFilters)

		// Use fileutil to handle temporary file creation
		tempFileHandle, err := fileutil.CreateTempFromFormFile(c, "cyclonedx-bom-file")
		if err != nil {
			logging.Error("Failed to create temporary file: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": "Failed to retrieve the BOM file",
			})
		}
		defer func() { _ = tempFileHandle.Cleanup() }()

		logging.Info("Processing BOM file: %s", tempFileHandle.Path)

		// Detect BOM format and validate
		bomFormat, err := validation.DetectBOMFormat(tempFileHandle.Path)
		if err != nil {
			logging.Warn("SBOM validation failed: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
			})
		}

		switch bomFormat {
		case "cyclonedx":
			if err := validation.ValidateCycloneDXBOM(tempFileHandle.Path); err != nil {
				logging.Warn("SBOM validation failed: %v", err)
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error": err.Error(),
				})
			}
		case "spdx":
			if err := validation.ValidateSPDXBOM(tempFileHandle.Path); err != nil {
				logging.Warn("SBOM validation failed: %v", err)
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error": err.Error(),
				})
			}
		}

		var bom *cyclonedx.Bom
		switch bomFormat {
		case "spdx":
			bom, err = spdx.ParseBOM(tempFileHandle.Path)
		default:
			bom, err = cyclonedx.ParseBOM(tempFileHandle.Path)
		}
		if err != nil {
			logging.Error("Failed to parse BOM file: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to parse BOM file",
			})
		}

		// Build dependency graph (non-fatal - CVE results still shown if Dgraph is unavailable)
		dgraphAvailable := true
		if err := deps.Repo.DropAllData(ctx); err != nil {
			logging.Warn("Failed to drop existing data (Dgraph may be unavailable): %v", err)
			dgraphAvailable = false
		}

		if dgraphAvailable {
			client := dgraph.DgraphClient()
			if err := dgraph.SetupSchema(client); err != nil {
				logging.Warn("Failed to setup schema: %v", err)
				dgraphAvailable = false
			}
		}

		if dgraphAvailable {
			if err := deps.Repo.InsertComponents(ctx, bom); err != nil {
				logging.Warn("Failed to insert BOM data into Dgraph: %v", err)
				dgraphAvailable = false
			}
		}

		trivyResults, err := trivy.RunTrivy(tempFileHandle.Path)
		if err != nil {
			logging.Error("Failed to run Trivy on %s: %v", tempFileHandle.Path, err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to run Trivy scanner",
			})
		}

		cvePurlMap, err := processor.ProcessFileInput(trivyResults)
		if err != nil {
			logging.Error("Failed to process Trivy results: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to process uploaded file",
			})
		}

		if dgraphAvailable {
			if err := deps.Repo.UpdateVulnerabilities(ctx, cvePurlMap); err != nil {
				logging.Warn("Failed to update vulnerabilities: %v", err)
			}
		}

		aggregatedData := nvd.AggregateCVEData(cvePurlMap)

		resultMap := make(map[string]dgraph.Component)
		if dgraphAvailable {
			var pathErr error
			resultMap, pathErr = deps.Repo.RetrieveVulnerablePURLs(ctx, cvePurlMap)
			if pathErr != nil {
				logging.Warn("Failed to retrieve vulnerable PURLs: %v", pathErr)
				resultMap = make(map[string]dgraph.Component)
			}
		}

		var cvePURLPairs []nvd.CVEPURLPair
		for _, data := range aggregatedData {
			// Filter by severity
			if !filterBySeverity(data, severityFilters) {
				continue
			}

			purl := strings.TrimSpace(strings.ToLower(data.PURL))

			for _, component := range resultMap {
				componentPurl := strings.TrimSpace(strings.ToLower(component.Purl))
				if componentPurl == purl {
					data.DgraphData = component
					break
				}
			}
			cvePURLPairs = append(cvePURLPairs, data)
		}

		return c.Render(http.StatusOK, "cve_vulnerability_results.html", map[string]interface{}{
			"CVEPURLPairs":    cvePURLPairs,
			"Components":      bom.Components,
			"SeverityFilters": severityFilters,
			"TotalVulns":      len(aggregatedData),
			"FilteredVulns":   len(cvePURLPairs),
		})
	}
}

// PurlTraversal handles PURL traversal requests
// This function requires HandlerDependencies to be injected
func PurlTraversal(deps *HandlerDependencies) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		pURL := c.QueryParam("pURL")
		if pURL == "" {
			logging.Error("pURL parameter is required but not provided")
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": "pURL is required",
			})
		}
		logging.Info("Processing PURL traversal for: %s", pURL)

		// Create temporary file from request body
		tempFileHandle, err := CreateTempFromRequestBody(c, ".json")
		if err != nil {
			logging.Error("Failed to create temporary file from request body: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error": "Invalid request body",
			})
		}
		defer func() { _ = tempFileHandle.Cleanup() }()

		// Drop existing data and setup schema
		if err := deps.Repo.DropAllData(ctx); err != nil {
			logging.Error("Error clearing data: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to clear database",
			})
		}

		// Re-setup schema after dropping all data
		client := dgraph.DgraphClient()
		if err := dgraph.SetupSchema(client); err != nil {
			logging.Error("Failed to setup schema: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to initialize database schema",
			})
		}

		bom, err := cyclonedx.ParseBOM(tempFileHandle.Path)
		if err != nil {
			logging.Error("Failed to parse BOM file: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to parse BOM file",
			})
		}

		if err := deps.Repo.InsertComponents(ctx, bom); err != nil {
			logging.Error("Failed to insert BOM data into Dgraph: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to insert BOM data into Dgraph",
			})
		}

		resultMap, err := deps.Repo.RetrievePURL(ctx, pURL)
		if err != nil {
			logging.Error("Error retrieving component for pURL %s: %v", pURL, err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to retrieve component data",
			})
		}

		logging.Debug("Retrieved data for pURL %s: %+v", pURL, resultMap)

		return c.JSON(http.StatusOK, resultMap)
	}
}

// CreateTempFromRequestBody creates a temporary file from the request body
func CreateTempFromRequestBody(c echo.Context, fileExtension string) (*fileutil.TempFileResult, error) {
	if c.Request().Body == nil {
		return nil, fmt.Errorf("request body is nil")
	}

	tempFile, err := os.CreateTemp("", "upload-*"+fileExtension)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	bytesWritten, err := io.Copy(tempFile, c.Request().Body)
	if err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to copy request body: %w", err)
	}

	if bytesWritten == 0 {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return nil, fmt.Errorf("request body is empty")
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to close temporary file: %w", err)
	}

	cleanup := func() error {
		return os.Remove(tempFile.Name())
	}

	return &fileutil.TempFileResult{
		Path:    tempFile.Name(),
		Cleanup: cleanup,
	}, nil
}
