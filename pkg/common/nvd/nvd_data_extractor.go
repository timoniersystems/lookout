// Package nvd provides utilities for fetching and processing CVE data
// from the National Vulnerability Database (NVD) API.
package nvd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"lookout/pkg/graph"
)

// Vulnerability represents a single vulnerability entry from the NVD API.
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

// CVEData represents the response structure from the NVD API.
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
	DgraphData graph.Component `json:"dgraphData"`
	Data       CVEData         `json:"data"`
	PURL       string          `json:"purl,omitempty"`
}

type AggregatedCVEData struct {
	CVEs []CVEPURLPair `json:"cves"`
}

// FetchCVEDataWithPURLs fetches CVE data for multiple CVE IDs with their associated PURLs.
// Continues processing even if individual CVE fetches fail, logging errors for failed fetches.
// Returns a slice of CVEPURLPair structs containing the CVE data and PURL.
func FetchCVEDataWithPURLs(cvePurlMap map[string]string) ([]CVEPURLPair, error) {
	var pairs []CVEPURLPair

	for cveID, purl := range cvePurlMap {
		data, err := FetchCVEData(cveID)
		if err != nil {
			log.Printf("Error fetching CVE data for %s: %v", cveID, err)
			// Continue processing other CVEs instead of failing entirely
			continue
		}
		pair := CVEPURLPair{
			Data: data,
			PURL: purl,
		}
		pairs = append(pairs, pair)
	}

	return pairs, nil
}

// FetchCVEData fetches CVE data from the NVD API for a single CVE ID.
// Implements smart retry logic with rate limit handling and exponential backoff.
// Supports NVD_API_KEY environment variable for higher rate limits (50 req/30s vs 5 req/30s).
// Injects default metrics (N/A values) for CVEs without CVSS v3.1 scores.
// Returns CVEData and error. The error will be non-nil if all retry attempts fail.
func FetchCVEData(cveID string) (CVEData, error) {
	url := fmt.Sprintf("https://services.nvd.nist.gov/rest/json/cves/2.0?cveid=%s", strings.ToUpper(cveID))

	var data CVEData
	var err error

	// NVD recommends 6 second delay between requests without API key
	// With API key: 0.6 second delay (50 requests per 30 seconds)
	maxRetries := 3 // Reduced from 5 to avoid excessive waiting
	baseDelay := 6 * time.Second

	// Check for API key
	apiKey := os.Getenv("NVD_API_KEY")
	if apiKey != "" {
		baseDelay = 600 * time.Millisecond // 0.6 seconds with API key
		log.Printf("Using NVD API key (rate limit: 50 req/30s)")
	}

	// Create HTTP client with timeout to prevent hanging
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create request with optional API key
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return data, fmt.Errorf("failed to create request: %w", err)
		}
		if apiKey != "" {
			req.Header.Set("apiKey", apiKey)
		}

		var resp *http.Response
		resp, err = httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()

			// Handle different HTTP status codes
			switch resp.StatusCode {
			case 200:
				// Success - process the response
				var body []byte
				body, err = io.ReadAll(resp.Body)
				if err == nil {
					err = json.Unmarshal(body, &data)
					if err != nil {
						// Log the actual response to help debug JSON parsing issues
						preview := string(body)
						if len(preview) > 200 {
							preview = preview[:200] + "..."
						}
						log.Printf("Failed to parse JSON response for %s. Response: %s", cveID, preview)
					}
					if err == nil {
						if len(data.Vulnerabilities) == 0 {
							return data, fmt.Errorf("CVE %s exists but has no vulnerability data", cveID)
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

			case 404:
				// CVE not found - don't retry
				return data, fmt.Errorf("CVE %s not found in NVD database (404)", cveID)

			case 403:
				// Forbidden - likely API key issue
				bodyBytes, _ := io.ReadAll(resp.Body)
				return data, fmt.Errorf("access forbidden (403) - check API key. Response: %s", string(bodyBytes))

			case 429:
				// Rate limited - check for Retry-After header
				retryAfter := resp.Header.Get("Retry-After")
				waitTime := baseDelay * time.Duration(attempt) // Exponential-ish backoff
				if retryAfter != "" {
					if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
						waitTime = time.Duration(seconds) * time.Second
					}
				}
				// Cap wait time at 30 seconds to avoid excessive delays
				if waitTime > 30*time.Second {
					waitTime = 30 * time.Second
				}
				if attempt < maxRetries {
					log.Printf("Rate limited (429) for %s. Waiting %v before retry %d/%d...", cveID, waitTime, attempt+1, maxRetries)
					time.Sleep(waitTime)
				}
				continue

			case 500, 502, 503, 504:
				// Server errors - retry with backoff
				log.Printf("Server error (%d) for %s, will retry...", resp.StatusCode, cveID)
				// Will retry with exponential backoff below

			default:
				// Other client errors (400, 401, etc.) - don't retry
				bodyBytes, _ := io.ReadAll(resp.Body)
				return data, fmt.Errorf("NVD API returned status %d: %s", resp.StatusCode, string(bodyBytes))
			}
		}

		// Calculate retry delay with exponential backoff and jitter
		if attempt < maxRetries {
			delay := baseDelay * time.Duration(1<<uint(attempt-1)) // Exponential: 6s, 12s, 24s, 48s...
			jitter := time.Duration(rand.Int63n(int64(delay / 4)))  // Add up to 25% jitter
			totalDelay := delay + jitter

			// Cap maximum delay at 2 minutes
			if totalDelay > 2*time.Minute {
				totalDelay = 2 * time.Minute
			}

			if err != nil {
				log.Printf("Attempt %d/%d failed for %s: %v. Retrying in %v...", attempt, maxRetries, cveID, err, totalDelay)
			}
			time.Sleep(totalDelay)
		}
	}

	return data, fmt.Errorf("failed to fetch CVE data for %s after %d attempts: %w", cveID, maxRetries, err)
}

// AggregateCVEData fetches and aggregates CVE data for multiple CVE IDs.
// Filters results to only include HIGH, CRITICAL, or N/A severity vulnerabilities.
// Continues processing even if individual CVE fetches fail.
func AggregateCVEData(cvePurlMap map[string]string) []CVEPURLPair {
	var pairs []CVEPURLPair
	for cveID, purl := range cvePurlMap {
		data, err := FetchCVEData(cveID)

		if err != nil {
			log.Printf("Error fetching CVE data for %s: %v", cveID, err)
			// Continue processing other CVEs instead of failing entirely
			continue
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
