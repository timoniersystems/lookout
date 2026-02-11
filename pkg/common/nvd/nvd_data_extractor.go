package nvd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"defender/pkg/gui/dgraph"
)

type Vulnerability struct {
	CVE struct {
		ID           string `json:"id"`
		Published    string `json:"published"`
		LastModified string `json:"lastModified"`
		VulnStatus   string `json:"vulnStatus"`
		Descriptions []struct {
			Lang  string `json:"lang"`
			Value string `json:"value"`
		} `json:"descriptions"`
		Metrics struct {
			CvssMetricV31 []struct {
				CvssData struct {
					Version      string  `json:"version"`
					VectorString string  `json:"vectorString"`
					BaseScore    float64 `json:"baseScore"`
					BaseSeverity string  `json:"baseSeverity"`
				} `json:"cvssData"`
			} `json:"cvssMetricV31"`
		} `json:"metrics"`
		Configurations []struct {
			Nodes []struct {
				CPEMatch []struct {
					Vulnerable            bool   `json:"vulnerable"`
					Criteria              string `json:"criteria"`
					VersionStartIncluding string `json:"versionStartIncluding,omitempty"`
					VersionStartExcluding string `json:"versionStartExcluding,omitempty"`
					VersionEndIncluding   string `json:"versionEndIncluding,omitempty"`
					VersionEndExcluding   string `json:"versionEndExcluding,omitempty"`
				} `json:"cpeMatch"`
			} `json:"nodes"`
		} `json:"configurations"`
		References []struct {
			URL string `json:"url"`
		} `json:"references"`
	} `json:"cve"`
}

type CVEData struct {
	ResultsPerPage  int             `json:"resultsPerPage"`
	StartIndex      int             `json:"startIndex"`
	TotalResults    int             `json:"totalResults"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
}

type ResultsPageData struct {
	CVEPURLPairs []CVEPURLPair
}

type CVEPURLPair struct {
	DgraphData dgraph.Component `json:"dgraphData"`
	Data       CVEData          `json:"data"`
	PURL       string           `json:"purl,omitempty"`
}

type AggregatedCVEData struct {
	CVEs []CVEPURLPair `json:"cves"`
}

func FetchCVEDataWithPURLs(cvePurlMap map[string]string) ([]CVEPURLPair, error) {
	var pairs []CVEPURLPair

	for cveID, purl := range cvePurlMap {
		data, err := FetchCVEData(cveID)
		if err != nil {
			fmt.Printf("Error with the CVE Data.")

		}
		pair := CVEPURLPair{
			Data: data,
			PURL: purl,
		}
		pairs = append(pairs, pair)
	}

	return pairs, nil
}

func FetchCVEData(cveID string) (CVEData, error) {
	url := fmt.Sprintf("https://services.nvd.nist.gov/rest/json/cves/2.0?cveid=%s", strings.ToUpper(cveID))

	var data CVEData
	var err error

	maxRetries := 20
	retryDelay := 5 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		var resp *http.Response
		resp, err = http.Get(url)
		if err == nil {
			defer resp.Body.Close()
			var body []byte
			body, err = io.ReadAll(resp.Body)
			if err == nil {
				err = json.Unmarshal(body, &data)
				if err == nil {
					if len(data.Vulnerabilities) == 0 {
						return data, fmt.Errorf("Error Extracting Data. CVE ID: %s was correctly formatted, but does not exist.\n", cveID)
					}
					for i, vulnerability := range data.Vulnerabilities {
						if len(vulnerability.CVE.Metrics.CvssMetricV31) == 0 {
							data.Vulnerabilities[i].CVE.Metrics.CvssMetricV31 = []struct {
								CvssData struct {
									Version      string  `json:"version"`
									VectorString string  `json:"vectorString"`
									BaseScore    float64 `json:"baseScore"`
									BaseSeverity string  `json:"baseSeverity"`
								} `json:"cvssData"`
							}{{
								CvssData: struct {
									Version      string  `json:"version"`
									VectorString string  `json:"vectorString"`
									BaseScore    float64 `json:"baseScore"`
									BaseSeverity string  `json:"baseSeverity"`
								}{
									Version:      "N/A",
									VectorString: "N/A",
									BaseScore:    0,
									BaseSeverity: "N/A",
								},
							}}
						}
					}
					return data, nil
				}
			}
		}

		fmt.Printf("Attempt %d: failed to fetch CVE data for %s: %v", attempt, cveID, err)
		time.Sleep(retryDelay)
		retryDelay *= 2
	}

	log.Fatalf("Failed to fetch CVE data after %d attempts: %v", maxRetries, err)
	return data, nil
}

func AggregateCVEData(cvePurlMap map[string]string) []CVEPURLPair {
	var pairs []CVEPURLPair
	for cveID, purl := range cvePurlMap {
		data, err := FetchCVEData(cveID)

		if err != nil {
			fmt.Printf("Error with the CVE Data.")
		}

		filteredVulnerabilities := []Vulnerability{}
		for _, vulnerability := range data.Vulnerabilities {
			for _, cvssMetric := range vulnerability.CVE.Metrics.CvssMetricV31 {
				severity := strings.ToLower(cvssMetric.CvssData.BaseSeverity)
				if severity == "high" || severity == "critical" || severity == "n/a" {
					filteredVulnerabilities = append(filteredVulnerabilities, vulnerability)
					break
				}
			}
		}

		if len(filteredVulnerabilities) > 0 {
			data.Vulnerabilities = filteredVulnerabilities
			pair := CVEPURLPair{
				Data: data,
				PURL: purl,
			}
			pairs = append(pairs, pair)
		}
	}

	return pairs
}

func outputToJsonFile(pair CVEPURLPair) error {
	for _, vulnerability := range pair.Data.Vulnerabilities {
		fileName := fmt.Sprintf("%s.json", vulnerability.CVE.ID)
		jsonData, err := json.MarshalIndent(pair, "", "  ")
		if err != nil {
			return err
		}

		err = os.WriteFile(fileName, jsonData, 0644)
		if err != nil {
			return err
		}

		fmt.Printf("CVE data for %s has been saved to %s\n", vulnerability.CVE.ID, fileName)
		return nil
	}

	if len(pair.Data.Vulnerabilities) == 0 {
		return fmt.Errorf("No vulnerabilities found to save to JSON")
	}

	return nil

}
