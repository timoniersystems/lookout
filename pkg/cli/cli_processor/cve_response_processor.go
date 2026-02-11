package cli_processor

import (
	"defender/pkg/common/nvd"
	"fmt"
	"strings"
)

func ProcessCVEData(cveIDs []string) {
	for _, cveID := range cveIDs {
		fmt.Printf("Fetching data for CVE: %s\n", cveID)
		data, err := nvd.FetchCVEData(cveID)

		if err != nil {
			fmt.Printf(err.Error())
			return
		}

		if len(data.Vulnerabilities) == 0 {
			fmt.Printf("No high or critical data found for CVE ID: %s\n", cveID)
			continue
		}

		for _, vulnerability := range data.Vulnerabilities {
			for _, cvssMetric := range vulnerability.CVE.Metrics.CvssMetricV31 {
				severity := strings.ToLower(strings.TrimSpace(cvssMetric.CvssData.BaseSeverity))
				if severity == "high" || severity == "critical" || severity == "n/a" {
					fmt.Printf("CVE ID: %s\n", vulnerability.CVE.ID)
					fmt.Printf("Published: %s\n", vulnerability.CVE.Published)
					fmt.Printf("Last Modified: %s\n", vulnerability.CVE.LastModified)
					fmt.Printf("Description: %s\n", vulnerability.CVE.Descriptions[0].Value)
					fmt.Printf("Base Severity: %s\n", cvssMetric.CvssData.BaseSeverity)
					fmt.Printf("Base Score: %f\n", cvssMetric.CvssData.BaseScore)

					fmt.Println("Configurations:")
					for _, config := range vulnerability.CVE.Configurations {
						for _, node := range config.Nodes {
							for _, cpeMatch := range node.CPEMatch {
								fmt.Printf("\t%s\n", cpeMatch.Criteria)
								if cpeMatch.VersionStartIncluding != "" {
									fmt.Printf("\t \t Version Start Including: %s\n", cpeMatch.VersionStartIncluding)
								}
								if cpeMatch.VersionStartExcluding != "" {
									fmt.Printf("\t \t Version Start Excluding: %s\n", cpeMatch.VersionStartExcluding)
								}
								if cpeMatch.VersionEndIncluding != "" {
									fmt.Printf("\t \t Version End Including: %s\n", cpeMatch.VersionEndIncluding)
								}
								if cpeMatch.VersionEndExcluding != "" {
									fmt.Printf("\t \t Version End Excluding: %s\n", cpeMatch.VersionEndExcluding)
								}
								fmt.Println("\t--------------------------------------------------------------")
							}
						}
					}
					fmt.Printf("References URLS: \n")
					for _, references := range vulnerability.CVE.References {
						fmt.Printf("%s\n", references.URL)
					}

					fmt.Println("***************************************************************************************************")
				} else {
					fmt.Printf("CVE with ID: %s is of the severity: %s so information will not be displayed as it must be HIGH, CRITICAL, or N/A.\n", cveID, severity)
					fmt.Println("***************************************************************************************************")
				}
			}

		}
	}

}

func ProcessCVEDataWithPURLs(pairs []nvd.CVEPURLPair) {
	for _, pair := range pairs {
		if len(pair.Data.Vulnerabilities) == 0 {
			continue
		}

		for _, vulnerability := range pair.Data.Vulnerabilities {
			for _, cvssMetric := range vulnerability.CVE.Metrics.CvssMetricV31 {
				severity := strings.ToLower(strings.TrimSpace(cvssMetric.CvssData.BaseSeverity))
				if severity == "high" || severity == "critical" || severity == "n/a" {
					fmt.Printf("CVE ID: %s\n", vulnerability.CVE.ID)
					fmt.Printf("PURL: %s\n", pair.PURL)
					fmt.Printf("Published: %s\n", vulnerability.CVE.Published)
					fmt.Printf("Last Modified: %s\n", vulnerability.CVE.LastModified)
					fmt.Printf("Description: %s\n", vulnerability.CVE.Descriptions[0].Value)
					fmt.Printf("Base Severity: %s\n", cvssMetric.CvssData.BaseSeverity)
					fmt.Printf("Base Score: %f\n", cvssMetric.CvssData.BaseScore)

					fmt.Println("Configurations:")
					for _, config := range vulnerability.CVE.Configurations {
						for _, node := range config.Nodes {
							for _, cpeMatch := range node.CPEMatch {
								fmt.Printf("\t%s\n", cpeMatch.Criteria)
								if cpeMatch.VersionStartIncluding != "" {
									fmt.Printf("\t \t Version Start Including: %s\n", cpeMatch.VersionStartIncluding)
								}
								if cpeMatch.VersionStartExcluding != "" {
									fmt.Printf("\t \t Version Start Excluding: %s\n", cpeMatch.VersionStartExcluding)
								}
								if cpeMatch.VersionEndIncluding != "" {
									fmt.Printf("\t \t Version End Including: %s\n", cpeMatch.VersionEndIncluding)
								}
								if cpeMatch.VersionEndExcluding != "" {
									fmt.Printf("\t \t Version End Excluding: %s\n", cpeMatch.VersionEndExcluding)
								}
								fmt.Println("\t--------------------------------------------------------------")
							}
						}
					}
					fmt.Printf("References URLS: \n")
					for _, references := range vulnerability.CVE.References {
						fmt.Printf("%s\n", references.URL)
					}

					fmt.Println("***************************************************************************************************")
				} else {
					fmt.Printf("CVE with ID: %s is of the severity: %s so information will not be displayed as it must be HIGH, CRITICAL, or N/A.\n", vulnerability.CVE.ID, severity)
					fmt.Println("***************************************************************************************************")
				}
			}
		}
	}
}
