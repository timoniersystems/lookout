package cli_processor

import (
	"lookout/pkg/common/nvd"
	"fmt"
	"io"
	"os"
	"strings"
)

// ANSI color codes for terminal output
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
	colorGreen   = "\033[32m"
	colorCyan    = "\033[36m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
)

// Box drawing characters
const (
	boxTopLeft     = "┌"
	boxTopRight    = "┐"
	boxBottomLeft  = "└"
	boxBottomRight = "┘"
	boxHorizontal  = "─"
	boxVertical    = "│"
	boxTee         = "├"
	boxTeeRight    = "┤"
)

// CVEFormatter handles formatted output of CVE data
type CVEFormatter struct {
	writer         io.Writer
	severityFilter string // "all", "critical", "high", "medium", "low"
}

// NewCVEFormatter creates a new CVE formatter writing to the specified writer
func NewCVEFormatter(writer io.Writer, severityFilter string) *CVEFormatter {
	if severityFilter == "" {
		severityFilter = "high" // Default to high
	}
	return &CVEFormatter{
		writer:         writer,
		severityFilter: strings.ToLower(severityFilter),
	}
}

// NewDefaultFormatter creates a formatter that writes to stdout with default severity filter
func NewDefaultFormatter() *CVEFormatter {
	return &CVEFormatter{
		writer:         os.Stdout,
		severityFilter: "high",
	}
}

// NewDefaultFormatterWithSeverity creates a formatter with custom severity filter
func NewDefaultFormatterWithSeverity(severityFilter string) *CVEFormatter {
	return NewCVEFormatter(os.Stdout, severityFilter)
}

// FormatVulnerability formats and writes a single vulnerability
func (f *CVEFormatter) FormatVulnerability(vulnerability nvd.Vulnerability, purl string) {
	for _, cvssMetric := range vulnerability.CVE.Metrics.CvssMetricV31 {
		severity := strings.ToLower(strings.TrimSpace(cvssMetric.CvssData.BaseSeverity))

		if f.shouldDisplaySeverity(severity) {
			f.formatHighSeverityVulnerability(vulnerability, cvssMetric, purl)
		} else {
			f.formatLowSeverityMessage(vulnerability.CVE.ID, severity)
		}
	}
}

// formatHighSeverityVulnerability formats high/critical severity vulnerabilities
func (f *CVEFormatter) formatHighSeverityVulnerability(
	vulnerability nvd.Vulnerability,
	cvssMetric struct {
		CvssData struct {
			Version      string  `json:"version"`
			VectorString string  `json:"vectorString"`
			BaseScore    float64 `json:"baseScore"`
			BaseSeverity string  `json:"baseSeverity"`
		} `json:"cvssData"`
	},
	purl string,
) {
	severity := strings.ToUpper(strings.TrimSpace(cvssMetric.CvssData.BaseSeverity))
	severityColor := f.getSeverityColor(severity)

	// Top border - matching dependency path style
	fmt.Fprintln(f.writer)
	fmt.Fprintf(f.writer, "%s\n", strings.Repeat("═", 70))
	fmt.Fprintf(f.writer, "  %sCVE ANALYSIS%s - %s%s%s%s\n",
		colorBold, colorReset,
		severityColor, colorBold, severity, colorReset)
	fmt.Fprintf(f.writer, "%s\n", strings.Repeat("═", 70))
	fmt.Fprintln(f.writer)

	// CVE ID with icon
	icon := f.getSeverityIcon(severity)
	fmt.Fprintf(f.writer, "  %s  %s%s%s%s\n",
		icon,
		colorBold, colorCyan, vulnerability.CVE.ID, colorReset)

	// PURL if provided
	if purl != "" {
		fmt.Fprintf(f.writer, "  %s📦  Package:%s %s\n",
			colorDim, colorReset, purl)
	}

	fmt.Fprintln(f.writer)

	// Severity and Score
	fmt.Fprintf(f.writer, "  %sSeverity:%s    %s\n",
		colorBold, colorReset, f.coloredSeverity(severity))
	fmt.Fprintf(f.writer, "  %sScore:%s       %.1f/10.0\n",
		colorBold, colorReset, cvssMetric.CvssData.BaseScore)
	fmt.Fprintf(f.writer, "  %sPublished:%s   %s\n",
		colorDim, colorReset, vulnerability.CVE.Published)
	fmt.Fprintf(f.writer, "  %sModified:%s    %s\n",
		colorDim, colorReset, vulnerability.CVE.LastModified)

	fmt.Fprintln(f.writer)

	// Separator
	fmt.Fprintf(f.writer, "%s\n", strings.Repeat("─", 70))
	fmt.Fprintln(f.writer)

	// Description
	fmt.Fprintf(f.writer, "  %sDescription%s\n", colorBold, colorReset)
	fmt.Fprintln(f.writer)

	description := "N/A"
	if len(vulnerability.CVE.Descriptions) > 0 {
		description = vulnerability.CVE.Descriptions[0].Value
	}
	f.printWrappedSimple(description, 66)

	// Configurations
	if len(vulnerability.CVE.Configurations) > 0 {
		fmt.Fprintln(f.writer)
		fmt.Fprintf(f.writer, "%s\n", strings.Repeat("─", 70))
		fmt.Fprintln(f.writer)
		f.formatConfigurations(vulnerability.CVE.Configurations)
	}

	// References
	if len(vulnerability.CVE.References) > 0 {
		fmt.Fprintln(f.writer)
		fmt.Fprintf(f.writer, "%s\n", strings.Repeat("─", 70))
		fmt.Fprintln(f.writer)
		f.formatReferences(vulnerability.CVE.References)
	}

	// Bottom border
	fmt.Fprintln(f.writer)
	fmt.Fprintf(f.writer, "%s\n", strings.Repeat("═", 70))
	fmt.Fprintln(f.writer)
}

// formatConfigurations formats configuration information
func (f *CVEFormatter) formatConfigurations(configurations []struct {
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
}) {
	fmt.Fprintf(f.writer, "  %sAffected Configurations%s\n", colorBold, colorReset)
	fmt.Fprintln(f.writer)

	count := 0
	for _, config := range configurations {
		for _, node := range config.Nodes {
			for _, cpeMatch := range node.CPEMatch {
				count++
				if count > 5 {
					fmt.Fprintf(f.writer, "     %s... and more (showing first 5)%s\n",
						colorDim, colorReset)
					return
				}

				// Parse CPE for better display
				cpe := f.formatCPE(cpeMatch.Criteria)
				fmt.Fprintf(f.writer, "     • %s%s%s\n",
					colorCyan, cpe, colorReset)

				// Version constraints
				if cpeMatch.VersionStartIncluding != "" {
					fmt.Fprintf(f.writer, "       %s≥ %s%s\n",
						colorDim, cpeMatch.VersionStartIncluding, colorReset)
				}
				if cpeMatch.VersionStartExcluding != "" {
					fmt.Fprintf(f.writer, "       %s> %s%s\n",
						colorDim, cpeMatch.VersionStartExcluding, colorReset)
				}
				if cpeMatch.VersionEndIncluding != "" {
					fmt.Fprintf(f.writer, "       %s≤ %s%s\n",
						colorDim, cpeMatch.VersionEndIncluding, colorReset)
				}
				if cpeMatch.VersionEndExcluding != "" {
					fmt.Fprintf(f.writer, "       %s< %s%s\n",
						colorDim, cpeMatch.VersionEndExcluding, colorReset)
				}
			}
		}
	}
}

// formatReferences formats reference URLs
func (f *CVEFormatter) formatReferences(references []struct {
	URL string `json:"url"`
}) {
	fmt.Fprintf(f.writer, "  %sReferences%s\n", colorBold, colorReset)
	fmt.Fprintln(f.writer)

	maxRefs := 5
	for i, reference := range references {
		if i >= maxRefs {
			remaining := len(references) - maxRefs
			fmt.Fprintf(f.writer, "     %s... and %d more%s\n",
				colorDim, remaining, colorReset)
			break
		}
		fmt.Fprintf(f.writer, "     • %s\n", reference.URL)
	}
}

// formatLowSeverityMessage formats a message for low-severity CVEs that are filtered out
func (f *CVEFormatter) formatLowSeverityMessage(cveID, severity string) {
	fmt.Fprintln(f.writer)
	fmt.Fprintf(f.writer, "%s ℹ️  %s %s(%s severity - filtered out)%s\n",
		colorDim, cveID,
		colorGreen, strings.ToUpper(severity), colorReset)
	fmt.Fprintln(f.writer)
}

// FormatCVEData formats complete CVE data
func (f *CVEFormatter) FormatCVEData(data nvd.CVEData, purl string) {
	if len(data.Vulnerabilities) == 0 {
		return
	}

	for _, vulnerability := range data.Vulnerabilities {
		f.FormatVulnerability(vulnerability, purl)
	}
}

// Helper functions

// shouldDisplaySeverity determines if a CVE with given severity should be displayed
// based on the configured severity filter
func (f *CVEFormatter) shouldDisplaySeverity(severity string) bool {
	severity = strings.ToLower(severity)
	filter := strings.ToLower(f.severityFilter)

	// "all" shows everything
	if filter == "all" {
		return true
	}

	// Map severity levels to numeric values for comparison
	severityLevel := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
		"n/a":      0,
	}

	filterLevel := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
	}

	currentLevel, ok := severityLevel[severity]
	if !ok {
		return true // Show unknown severities
	}

	minLevel, ok := filterLevel[filter]
	if !ok {
		minLevel = 3 // Default to "high" if invalid filter
	}

	return currentLevel >= minLevel
}

// getSeverityColor returns the appropriate color for a severity level
func (f *CVEFormatter) getSeverityColor(severity string) string {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return colorRed + colorBold
	case "HIGH":
		return colorRed
	case "MEDIUM":
		return colorYellow
	case "LOW":
		return colorGreen
	default:
		return colorReset
	}
}

// coloredSeverity returns a colored severity string
func (f *CVEFormatter) coloredSeverity(severity string) string {
	color := f.getSeverityColor(severity)
	return fmt.Sprintf("%s%s%s", color, severity, colorReset)
}

// truncate truncates a string to maxLen, adding "..." if truncated
func (f *CVEFormatter) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// printWrapped prints text wrapped to fit within the box with proper padding
func (f *CVEFormatter) printWrapped(text string, maxWidth int, indent string) {
	words := strings.Fields(text)
	if len(words) == 0 {
		fmt.Fprintf(f.writer, "%s%s%-*s %s\n", boxVertical, indent, maxWidth, "", boxVertical)
		return
	}

	line := ""
	for _, word := range words {
		testLine := line
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		if len(testLine) > maxWidth {
			// Print current line
			padding := maxWidth - len(line)
			fmt.Fprintf(f.writer, "%s%s%s%s %s\n",
				boxVertical, indent, line, strings.Repeat(" ", padding), boxVertical)
			line = word
		} else {
			line = testLine
		}
	}

	// Print remaining text
	if line != "" {
		padding := maxWidth - len(line)
		fmt.Fprintf(f.writer, "%s%s%s%s %s\n",
			boxVertical, indent, line, strings.Repeat(" ", padding), boxVertical)
	}
}

// formatCPE extracts readable information from CPE string
func (f *CVEFormatter) formatCPE(cpe string) string {
	// CPE format: cpe:2.3:a:vendor:product:version:...
	parts := strings.Split(cpe, ":")
	if len(parts) >= 5 {
		vendor := parts[3]
		product := parts[4]
		version := "*"
		if len(parts) > 5 && parts[5] != "*" {
			version = parts[5]
		}
		return fmt.Sprintf("%s/%s (%s)", vendor, product, version)
	}
	return cpe
}

// getSeverityIcon returns an appropriate icon for the severity level
func (f *CVEFormatter) getSeverityIcon(severity string) string {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return "🔴"
	case "HIGH":
		return "🟠"
	case "MEDIUM":
		return "🟡"
	case "LOW":
		return "🟢"
	default:
		return "⚪"
	}
}

// printWrappedSimple prints text wrapped to fit within maxWidth with simple indentation
func (f *CVEFormatter) printWrappedSimple(text string, maxWidth int) {
	words := strings.Fields(text)
	if len(words) == 0 {
		fmt.Fprintln(f.writer)
		return
	}

	indent := "    "
	line := ""
	for _, word := range words {
		testLine := line
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		if len(testLine) > maxWidth {
			// Print current line
			fmt.Fprintf(f.writer, "%s%s\n", indent, line)
			line = word
		} else {
			line = testLine
		}
	}

	// Print remaining text
	if line != "" {
		fmt.Fprintf(f.writer, "%s%s\n", indent, line)
	}
}
