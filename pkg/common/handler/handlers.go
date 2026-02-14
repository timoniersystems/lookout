package handler

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lookout/pkg/common/cyclonedx"
	"lookout/pkg/common/nvd"
	"lookout/pkg/common/processor"
	"lookout/pkg/common/trivy"
	"lookout/pkg/ui/dgraph"

	"github.com/labstack/echo/v4"
)

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
				log.Printf("[DEBUG] CVE %s has N/A severity, including because all severities selected", vuln.CVE.ID)
				return true
			}

			// Check if severity matches the selected filters
			if severityMap[severity] {
				log.Printf("[DEBUG] CVE %s (severity=%s) MATCHED filter", vuln.CVE.ID, severity)
				return true
			}
		} else {
			// No CVSS v3.1 metrics available
			if allSeveritiesSelected {
				log.Printf("[DEBUG] CVE %s has no CvssMetricV31 data, including because all severities selected", vuln.CVE.ID)
				return true
			}
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
			log.Printf("Error fetching CVE data: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to fetch CVE data",
			})
		}
		return c.Render(http.StatusOK, "cve_details.html", data)
	}

	file, err := c.FormFile("file")
	if err == nil && file != nil {
		src, err := file.Open()
		if err != nil {
			log.Printf("Failed to open the uploaded file: %v", err)
			return err
		}
		defer src.Close()

		tempFile, err := os.CreateTemp("", "upload-*"+filepath.Ext(file.Filename))
		if err != nil {
			log.Printf("Failed to create a temporary file: %v", err)
			return err
		}
		defer os.Remove(tempFile.Name())

		_, err = io.Copy(tempFile, src)
		if err != nil {
			log.Printf("Failed to copy the uploaded file to the temporary file: %v", err)
			return err
		}

		if err := tempFile.Close(); err != nil {
			log.Printf("Failed to close the temporary file: %v", err)
			return err
		}

		cveIDs, err := processor.ProcessFileInputForCVEs(tempFile.Name())
		if err != nil {
			log.Printf("Failed to extract CVE IDs from the uploaded file: %v", err)
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
		log.Printf("Error fetching CVE data: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to fetch CVE data",
		})
	}

	return c.Render(http.StatusOK, "results.html", data)
}

func UploadAndProcess(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("Failed to retrieve the file from request: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the file",
		})
	}

	src, err := file.Open()
	if err != nil {
		log.Printf("Failed to open the uploaded file: %v", err)
		return err
	}
	defer src.Close()

	ext := filepath.Ext(file.Filename)
	tempFile, err := os.CreateTemp("", "upload-*"+ext)
	if err != nil {
		log.Printf("Failed to create a temporary file: %v", err)
		return err
	}
	tempFilePath := tempFile.Name()
	log.Printf("Uploaded file saved to: %s", tempFilePath)
	defer os.Remove(tempFilePath)

	_, err = io.Copy(tempFile, src)
	if err != nil {
		log.Printf("Failed to copy the uploaded file to the temporary file: %v", err)
		return err
	}

	if err := tempFile.Close(); err != nil {
		log.Printf("Failed to close the temporary file: %v", err)
		return err
	}

	cvePurlMap, err := processor.ProcessFileInput(tempFilePath)
	if err != nil {
		log.Printf("Failed to process the uploaded file at %s: %v", tempFilePath, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to process uploaded file",
		})
	}

	aggregatedData := nvd.AggregateCVEData(cvePurlMap)

	var pageData nvd.ResultsPageData
	for _, data := range aggregatedData {
		pageData.CVEPURLPairs = append(pageData.CVEPURLPairs, data)
	}

	// Sort results by severity (CRITICAL → HIGH → MEDIUM → LOW → N/A) and score
	sortCVEPURLPairs(pageData.CVEPURLPairs)

	return c.Render(http.StatusOK, "cve_results.html", pageData)
}

func RunTrivyAndProcess(c echo.Context) error {
	if !trivy.CheckTrivyInstalled() {
		log.Println("Please install Trivy before running this application.")
		return c.JSON(http.StatusBadRequest, "error: Please install Trivy before running the application.")
	}

	// Get severity filters from form
	severityFilters := c.Request().Form["severity"]
	if len(severityFilters) == 0 {
		// Default to HIGH and CRITICAL if none selected
		severityFilters = []string{"CRITICAL", "HIGH"}
	}
	log.Printf("Severity filters: %v", severityFilters)

	file, err := c.FormFile("sbom-file")
	if err != nil {
		log.Printf("Failed to retrieve the file from request: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the file",
		})
	}

	src, err := file.Open()
	if err != nil {
		log.Printf("Failed to open the uploaded file: %v", err)
		return err
	}
	defer src.Close()

	ext := filepath.Ext(file.Filename)
	tempFile, err := os.CreateTemp("", "upload-*"+ext)
	if err != nil {
		log.Printf("Failed to create a temporary file: %v", err)
		return err
	}
	tempFilePath := tempFile.Name()
	log.Printf("Uploaded file saved to: %s", tempFilePath)
	defer os.Remove(tempFilePath)

	_, err = io.Copy(tempFile, src)
	if err != nil {
		log.Printf("Failed to copy the uploaded file to the temporary file: %v", err)
		return err
	}

	if err := tempFile.Close(); err != nil {
		log.Printf("Failed to close the temporary file: %v", err)
		return err
	}

	trivyResults, err := trivy.RunTrivy(tempFilePath)
	if err != nil {
		log.Printf("Failed to run Trivy on %s: %v", tempFilePath, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to run Trivy scanner",
		})
	}

	cvePurlMap, err := processor.ProcessFileInput(trivyResults)
	if err != nil {
		log.Printf("Failed to process the uploaded file at %s: %v", tempFilePath, err)
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

func UploadBOMAndInsertData(c echo.Context) error {
	// Get severity filters from form
	severityFilters := c.Request().Form["severity"]
	if len(severityFilters) == 0 {
		// Default to HIGH and CRITICAL if none selected
		severityFilters = []string{"CRITICAL", "HIGH"}
	}
	log.Printf("Severity filters: %v", severityFilters)

	client := dgraph.DgraphClient()
	if err := dgraph.DropAllData(client); err != nil {
		log.Printf("Failed to drop existing data: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to clear database",
		})
	}

	// Re-setup schema after dropping all data
	if err := dgraph.SetupSchema(client); err != nil {
		log.Printf("Failed to setup schema: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to initialize database schema",
		})
	}

	file, err := c.FormFile("cyclonedx-bom-file")
	if err != nil {
		log.Printf("Failed to retrieve the BOM file from request: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the BOM file",
		})
	}

	src, err := file.Open()
	if err != nil {
		log.Printf("Failed to open the uploaded BOM file: %v", err)
		return err
	}
	defer src.Close()

	tempFile, err := os.CreateTemp("", "bom-*"+filepath.Ext(file.Filename))
	if err != nil {
		log.Printf("Failed to create a temporary file for the BOM: %v", err)
		return err
	}
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, src)
	if err != nil {
		log.Printf("Failed to copy the BOM file to the temporary file: %v", err)
		return err
	}

	if err := tempFile.Close(); err != nil {
		log.Printf("Failed to close the temporary BOM file: %v", err)
		return err
	}

	bom, err := cyclonedx.ParseBOM(tempFile.Name())
	if err != nil {
		log.Printf("Failed to parse BOM file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to parse BOM file",
		})
	}

	if err := dgraph.InsertComponentsAndDependencies(dgraph.DgraphClient(), bom); err != nil {
		log.Printf("Failed to insert BOM data into Dgraph: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to insert BOM data into Dgraph",
		})
	}

	trivyResults, err := trivy.RunTrivy(tempFile.Name())
	if err != nil {
		log.Printf("Failed to run Trivy on %s: %v", tempFile.Name(), err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to run Trivy scanner",
		})
	}

	cvePurlMap, err := processor.ProcessFileInput(trivyResults)
	if err != nil {
		log.Printf("Failed to process the uploaded file at %s: %v", tempFile.Name(), err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to process uploaded file",
		})
	}

	dgraph.QueryAndUpdatePurl(cvePurlMap)
	aggregatedData := nvd.AggregateCVEData(cvePurlMap)

	resultMap, err := dgraph.RetrieveVulnerablePURLs(cvePurlMap)
	if err != nil {
		log.Printf("Failed to retrieve vulnerable PURLs: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve vulnerability data",
		})
	}

	var pageData nvd.ResultsPageData

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
		pageData.CVEPURLPairs = append(pageData.CVEPURLPairs, data)

	}

	return c.Render(http.StatusOK, "cve_vulnerability_results.html", pageData)
}

func PurlTraversal(c echo.Context) error {
	pURL := c.QueryParam("pURL")
	if pURL == "" {
		log.Println("Error: pURL is required")
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "pURL is required",
		})
	}
	log.Printf("Received pURL: %s", pURL)

	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	// Create temporary file for BOM
	tempFile, err := os.CreateTemp("", "upload-*.json")
	if err != nil {
		log.Printf("Failed to create temporary file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to create temporary file",
		})
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(body); err != nil {
		log.Printf("Failed to write to temporary file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to write JSON to file",
		})
	}

	if err := tempFile.Close(); err != nil {
		log.Printf("Failed to close temporary file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to close temporary file",
		})
	}

	// Use service layer for processing
	// Note: In a real application, the service should be injected as a dependency
	// For now, we'll create it here for backward compatibility
	client := dgraph.DgraphClient()
	err = dgraph.DropAllData(client)
	if err != nil {
		log.Printf("Error clearing data: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to clear database",
		})
	}

	// Re-setup schema after dropping all data
	if err := dgraph.SetupSchema(client); err != nil {
		log.Printf("Failed to setup schema: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to initialize database schema",
		})
	}

	bom, err := cyclonedx.ParseBOM(tempFile.Name())
	if err != nil {
		log.Printf("Failed to parse BOM file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to parse BOM file",
		})
	}

	if err := dgraph.InsertComponentsAndDependencies(dgraph.DgraphClient(), bom); err != nil {
		log.Printf("Failed to insert BOM data into Dgraph: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to insert BOM data into Dgraph",
		})
	}

	resultMap, err := dgraph.RetrievePURL(pURL)
	if err != nil {
		log.Printf("Error retrieving component for pURL %s: %v", pURL, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve component data",
		})
	}

	log.Printf("Final retrieved data for pURL %s: %v", pURL, resultMap)

	return c.JSON(http.StatusOK, resultMap)
}
