package dgraph_test

import (
	"defender/pkg/gui/dgraph"
	"net/url"
	"testing"
)

func TestGenerateQueryURL(t *testing.T) {
	t.Log("Starting TestGenerateQueryURL")
	urlStr, err := dgraph.GenerateQueryURL("pkg:npm/a@1.0.0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	t.Logf("Generated URL: %s", urlStr)
	if _, err := url.ParseRequestURI(urlStr); err != nil {
		t.Errorf("Invalid URL returned: %s", urlStr)
	}
	t.Log("Finished TestGenerateQueryURL")
}

func TestGenerateQueryURL_EmptyInput(t *testing.T) {
	t.Log("Starting TestGenerateQueryURL_EmptyInput")
	_, err := dgraph.GenerateQueryURL("")
	if err == nil {
		t.Error("Expected error for empty package name")
	} else {
		t.Logf("Received expected error: %v", err)
	}
	t.Log("Finished TestGenerateQueryURL_EmptyInput")
}
