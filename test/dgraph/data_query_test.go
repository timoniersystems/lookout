package dgraph_test

import (
	"testing"
)

func TestRetrieveVulnerablePURLs(t *testing.T) {
	vulns := map[string][]string{
		"CVE-2024-0001": {"pkg:npm/a@1.0.0"},
		"CVE-2024-0002": {},
	}
	if len(vulns["CVE-2024-0001"]) != 1 {
		t.Error("Expected 1 vulnerable PURL")
	}
	if len(vulns["CVE-2024-0002"]) != 0 {
		t.Error("Expected 0 vulnerable PURLs")
	}
}
