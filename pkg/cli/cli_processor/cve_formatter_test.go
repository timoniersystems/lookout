package cli_processor

import (
	"bytes"
	"github.com/timoniersystems/lookout/pkg/common/nvd"
	"strings"
	"testing"
)

func TestNewCVEFormatter(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := NewCVEFormatter(buf, "critical")

	if formatter.writer != buf {
		t.Error("Expected writer to be set correctly")
	}

	if formatter.severityFilter != "critical" {
		t.Errorf("Expected severityFilter to be 'critical', got %q", formatter.severityFilter)
	}
}

func TestNewCVEFormatter_EmptyFilter(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := NewCVEFormatter(buf, "")

	if formatter.severityFilter != "high" {
		t.Errorf("Expected default severity filter 'high', got %q", formatter.severityFilter)
	}
}

func TestNewDefaultFormatter(t *testing.T) {
	formatter := NewDefaultFormatter()

	if formatter.severityFilter != "high" {
		t.Errorf("Expected severity filter 'high', got %q", formatter.severityFilter)
	}
}

func TestNewDefaultFormatterWithSeverity(t *testing.T) {
	formatter := NewDefaultFormatterWithSeverity("critical")

	if formatter.severityFilter != "critical" {
		t.Errorf("Expected severity filter 'critical', got %q", formatter.severityFilter)
	}
}

func TestFormatVulnerability_HighSeverity(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := NewCVEFormatter(buf, "high")

	vulnerability := nvd.Vulnerability{
		CVE: struct {
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
		}{
			ID: "CVE-2021-44228",
			Descriptions: []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			}{
				{Lang: "en", Value: "Apache Log4j2 vulnerability"},
			},
			Metrics: struct {
				CvssMetricV31 []struct {
					CvssData struct {
						Version      string  `json:"version"`
						VectorString string  `json:"vectorString"`
						BaseScore    float64 `json:"baseScore"`
						BaseSeverity string  `json:"baseSeverity"`
					} `json:"cvssData"`
				} `json:"cvssMetricV31"`
			}{
				CvssMetricV31: []struct {
					CvssData struct {
						Version      string  `json:"version"`
						VectorString string  `json:"vectorString"`
						BaseScore    float64 `json:"baseScore"`
						BaseSeverity string  `json:"baseSeverity"`
					} `json:"cvssData"`
				}{
					{
						CvssData: struct {
							Version      string  `json:"version"`
							VectorString string  `json:"vectorString"`
							BaseScore    float64 `json:"baseScore"`
							BaseSeverity string  `json:"baseSeverity"`
						}{
							Version:      "3.1",
							VectorString: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
							BaseScore:    10.0,
							BaseSeverity: "CRITICAL",
						},
					},
				},
			},
		},
	}

	formatter.FormatVulnerability(vulnerability, "pkg:maven/org.apache.logging.log4j/log4j-core@2.14.1")

	output := buf.String()
	if !strings.Contains(output, "CVE-2021-44228") {
		t.Error("Expected output to contain CVE ID")
	}
	if !strings.Contains(output, "CRITICAL") {
		t.Error("Expected output to contain severity level")
	}
}

func TestFormatVulnerability_LowSeverity_Filtered(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := NewCVEFormatter(buf, "high")

	vulnerability := nvd.Vulnerability{
		CVE: struct {
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
		}{
			ID: "CVE-2021-12345",
			Metrics: struct {
				CvssMetricV31 []struct {
					CvssData struct {
						Version      string  `json:"version"`
						VectorString string  `json:"vectorString"`
						BaseScore    float64 `json:"baseScore"`
						BaseSeverity string  `json:"baseSeverity"`
					} `json:"cvssData"`
				} `json:"cvssMetricV31"`
			}{
				CvssMetricV31: []struct {
					CvssData struct {
						Version      string  `json:"version"`
						VectorString string  `json:"vectorString"`
						BaseScore    float64 `json:"baseScore"`
						BaseSeverity string  `json:"baseSeverity"`
					} `json:"cvssData"`
				}{
					{
						CvssData: struct {
							Version      string  `json:"version"`
							VectorString string  `json:"vectorString"`
							BaseScore    float64 `json:"baseScore"`
							BaseSeverity string  `json:"baseSeverity"`
						}{
							BaseScore:    3.5,
							BaseSeverity: "LOW",
						},
					},
				},
			},
		},
	}

	formatter.FormatVulnerability(vulnerability, "")

	output := buf.String()
	if !strings.Contains(output, "filtered") || !strings.Contains(output, "LOW") {
		t.Error("Expected output to indicate vulnerability was filtered due to low severity")
	}
}

func TestFormatCVEData_MultipleVulnerabilities(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := NewCVEFormatter(buf, "all")

	data := nvd.CVEData{
		Vulnerabilities: []nvd.Vulnerability{
			{
				CVE: struct {
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
				}{
					ID: "CVE-2021-00001",
					Metrics: struct {
						CvssMetricV31 []struct {
							CvssData struct {
								Version      string  `json:"version"`
								VectorString string  `json:"vectorString"`
								BaseScore    float64 `json:"baseScore"`
								BaseSeverity string  `json:"baseSeverity"`
							} `json:"cvssData"`
						} `json:"cvssMetricV31"`
					}{
						CvssMetricV31: []struct {
							CvssData struct {
								Version      string  `json:"version"`
								VectorString string  `json:"vectorString"`
								BaseScore    float64 `json:"baseScore"`
								BaseSeverity string  `json:"baseSeverity"`
							} `json:"cvssData"`
						}{
							{
								CvssData: struct {
									Version      string  `json:"version"`
									VectorString string  `json:"vectorString"`
									BaseScore    float64 `json:"baseScore"`
									BaseSeverity string  `json:"baseSeverity"`
								}{
									BaseSeverity: "HIGH",
									BaseScore:    7.5,
								},
							},
						},
					},
				},
			},
		},
	}

	formatter.FormatCVEData(data, "pkg:npm/test@1.0.0")

	output := buf.String()
	if !strings.Contains(output, "CVE-2021-00001") {
		t.Error("Expected output to contain CVE ID")
	}
}

func TestShouldDisplaySeverity(t *testing.T) {
	testCases := []struct {
		name           string
		filter         string
		severity       string
		shouldDisplay  bool
	}{
		{"All filter shows critical", "all", "critical", true},
		{"All filter shows high", "all", "high", true},
		{"All filter shows medium", "all", "medium", true},
		{"All filter shows low", "all", "low", true},
		{"High filter shows critical", "high", "critical", true},
		{"High filter shows high", "high", "high", true},
		{"High filter hides medium", "high", "medium", false},
		{"High filter hides low", "high", "low", false},
		{"Critical filter shows only critical", "critical", "critical", true},
		{"Critical filter hides high", "critical", "high", false},
		{"Medium filter shows high", "medium", "high", true},
		{"Medium filter shows medium", "medium", "medium", true},
		{"Medium filter hides low", "medium", "low", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formatter := NewCVEFormatter(&bytes.Buffer{}, tc.filter)
			result := formatter.shouldDisplaySeverity(tc.severity)
			if result != tc.shouldDisplay {
				t.Errorf("Expected shouldDisplaySeverity(%q, %q) = %v, got %v",
					tc.filter, tc.severity, tc.shouldDisplay, result)
			}
		})
	}
}

