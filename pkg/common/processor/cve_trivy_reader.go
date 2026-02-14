package processor

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

// TrivyResults represents the structure of Trivy JSON scan results.
type TrivyResults struct {
	Results []struct {
		Vulnerabilities []struct {
			VulnerabilityID string `json:"VulnerabilityID"`
			PkgIdentifier   struct {
				PURL string `json:"PURL"`
			} `json:"PkgIdentifier"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion     string `json:"FixedVersion"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

// ParseTrivyJSONFile parses a Trivy JSON scan results file and extracts CVE IDs with their PURLs.
// Only processes vulnerabilities with IDs starting with "CVE" (case-insensitive).
// Returns a map of CVE IDs to Package URLs.
func ParseTrivyJSONFile(filePath string) (map[string]string, error) {
	var trivy TrivyResults

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read the JSON file: %s, error: %v", filePath, err)
		return nil, err
	}

	if err := json.Unmarshal(data, &trivy); err != nil {
		log.Printf("Failed to unmarshal Trivy JSON data: %v", err)
		return nil, err
	}

	cvePurlMap := make(map[string]string)
	for _, result := range trivy.Results {
		for _, vuln := range result.Vulnerabilities {
			if strings.HasPrefix(strings.ToLower(vuln.VulnerabilityID), "cve") { //Temporary. Will change when we add other DBs in.
				cvePurlMap[vuln.VulnerabilityID] = vuln.PkgIdentifier.PURL
			} else {
				log.Printf("Skipping VulnerabilityID %s as it does not start with 'cve'", vuln.VulnerabilityID)
			}
		}
	}

	for purl, cve := range cvePurlMap {
		log.Printf("Here is the CVE ID: %s and corresponding PURL: %s", purl, cve)
	}

	return cvePurlMap, nil
}
