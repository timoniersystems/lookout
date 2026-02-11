package dgraph

import (
	"fmt"
	"net/url"
)

func GenerateQueryURL(packageName string) (string, error) {
	if packageName == "" {
		return "", fmt.Errorf("package name cannot be empty")
	}

	queryTemplate := `
      {
        component(func: eq(purl, "%s")) {
          uid
          reference
          name
          version
          decodedPURL
          vulnerable
          dgraphURL
          cveID
          bomRef
          dependsOn {
            uid
            reference
            name
            version
            decodedPURL
            vulnerable
            dgraphURL
            cveID
            bomRef
          }
        }
      }`

	query := fmt.Sprintf(queryTemplate, packageName)
	encodedQuery := url.QueryEscape(query)
	baseURL := "http://localhost:8000/?query="
	fullURL := baseURL + encodedQuery

	if _, err := url.ParseRequestURI(fullURL); err != nil {
		return "", fmt.Errorf("generated URL is invalid: %w", err)
	}

	return fullURL, nil
}
