package processor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ReadCVEIDsFromTextFile reads CVE IDs from a plain text file.
// Each line should contain a CVE ID, optionally followed by a comma and PURL.
// Format: CVE-YYYY-NNNNN[,pkg:type/name@version]
// Returns a map of CVE IDs to PURLs (empty string if no PURL provided).
func ReadCVEIDsFromTextFile(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	cvePurlMap := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")

		var purl string
		if len(parts) > 1 {
			purl = strings.TrimSpace(parts[1])
		} else {
			purl = ""
		}
		cveID := strings.TrimSpace(parts[0])
		cvePurlMap[cveID] = purl
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return cvePurlMap, nil
}
