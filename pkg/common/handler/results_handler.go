package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// GetSBOMResults retrieves and displays stored SBOM analysis results
func GetSBOMResults(c echo.Context) error {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Session ID required",
		})
	}

	results := GetResults(sessionID)
	if results == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Results not found or expired",
		})
	}

	// Render the unified results page with vulnerability and dependency path data
	return c.Render(http.StatusOK, "cve_vulnerability_results.html", map[string]interface{}{
		"CVEPURLPairs":     results.CVEPURLPairs,
		"Components":       results.Components,
		"SeverityFilters":  results.SeverityFilters,
		"TotalVulns":       results.TotalVulns,
		"FilteredVulns":    results.FilteredVulns,
	})
}
