// Package interfaces defines core abstractions for dependency injection and testing.
// These interfaces allow for easy mocking and testing of components.
package interfaces

import (
	"context"
	"io"
	"net/http"

	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
)

// HTTPClient defines an interface for making HTTP requests
type HTTPClient interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// TrivyRunner defines an interface for running Trivy scans
type TrivyRunner interface {
	// RunTrivy runs a Trivy scan on the specified SBOM path
	// Returns the path to the results file or error
	RunTrivy(sbomPath string, outputPaths ...string) (string, error)

	// CheckInstalled verifies if Trivy is installed and available
	CheckInstalled() bool
}

// CVEDataFetcher defines an interface for fetching CVE data
type CVEDataFetcher interface {
	// FetchCVEData retrieves CVE data for a single CVE ID
	FetchCVEData(cveID string) (interface{}, error)

	// FetchCVEDataWithPURLs retrieves CVE data for multiple CVE IDs with their PURLs
	FetchCVEDataWithPURLs(cvePurlMap map[string]string) (interface{}, error)
}

// FileProcessor defines an interface for processing different file formats
type FileProcessor interface {
	// ProcessFileInput processes a file and extracts CVE to PURL mappings
	ProcessFileInput(filePath string) (map[string]string, error)
}

// BOMParser defines an interface for parsing Software Bill of Materials
type BOMParser interface {
	// ParseBOM parses a BOM file and returns the BOM structure
	ParseBOM(filePath string) (*cyclonedx.Bom, error)
}

// VulnerabilityRepository defines an interface for managing vulnerability data in the database
type VulnerabilityRepository interface {
	// InsertComponents inserts components and their dependencies from a BOM
	InsertComponents(ctx context.Context, bom *cyclonedx.Bom) error

	// FindComponent finds a component by PURL and CVE ID
	FindComponent(ctx context.Context, purl, cveID string) (*dgraph.Component, error)

	// FindShortestPath finds the shortest path from source to root component
	FindShortestPath(ctx context.Context, sourceUID, rootUID string, maxDepth int) ([]string, error)

	// UpdateVulnerabilities updates vulnerability information for components
	UpdateVulnerabilities(ctx context.Context, cvePurlMap map[string]string) error

	// RetrieveVulnerablePURLs retrieves all vulnerable components for given CVE/PURL pairs
	RetrieveVulnerablePURLs(ctx context.Context, cvePurlMap map[string]string) (map[string]dgraph.Component, error)

	// RetrievePURL retrieves component information for a specific PURL
	RetrievePURL(purl string) (map[string]dgraph.Component, error)

	// DropAllData drops all data from the database (preserves schema)
	DropAllData(ctx context.Context) error

	// SetupSchema sets up the database schema
	SetupSchema(ctx context.Context) error
}

// Logger defines an interface for structured logging
type Logger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// OutputWriter defines an interface for writing formatted output
type OutputWriter interface {
	io.Writer

	// WriteFormatted writes formatted output for a specific data type
	WriteFormatted(data interface{}) error
}

// Validator defines an interface for input validation
type Validator interface {
	// ValidateCVEID validates a CVE ID format
	ValidateCVEID(cveID string) error

	// ValidatePURL validates a Package URL format
	ValidatePURL(purl string) error

	// ValidateFilePath validates a file path
	ValidateFilePath(path string) error
}
