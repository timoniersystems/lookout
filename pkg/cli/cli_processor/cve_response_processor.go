package cli_processor

import (
	"lookout/pkg/common/nvd"
	"fmt"
	"log"
)

// ProcessCVEData fetches and formats CVE data for the given CVE IDs
func ProcessCVEData(cveIDs []string, severityFilter string) {
	formatter := NewDefaultFormatterWithSeverity(severityFilter)

	for _, cveID := range cveIDs {
		fmt.Printf("Fetching data for CVE: %s\n", cveID)
		data, err := nvd.FetchCVEData(cveID)

		if err != nil {
			log.Printf("Error fetching CVE data: %v\n", err)
			return
		}

		if len(data.Vulnerabilities) == 0 {
			fmt.Printf("No vulnerabilities found for CVE ID: %s\n", cveID)
			continue
		}

		formatter.FormatCVEData(data, "")
	}
}

// ProcessCVEDataWithPURLs formats CVE data with associated PURLs
func ProcessCVEDataWithPURLs(pairs []nvd.CVEPURLPair, severityFilter string) {
	formatter := NewDefaultFormatterWithSeverity(severityFilter)

	for _, pair := range pairs {
		if len(pair.Data.Vulnerabilities) == 0 {
			continue
		}

		formatter.FormatCVEData(pair.Data, pair.PURL)
	}
}
