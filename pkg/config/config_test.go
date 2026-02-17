package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing env vars
	clearEnvVars()

	config, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Test default values
	if config.Server.Port != 3000 {
		t.Errorf("Expected default server port 3000, got %d", config.Server.Port)
	}

	if config.Dgraph.Host != "alpha" {
		t.Errorf("Expected default Dgraph host 'alpha', got %s", config.Dgraph.Host)
	}

	if config.Dgraph.Port != 9080 {
		t.Errorf("Expected default Dgraph port 9080, got %d", config.Dgraph.Port)
	}

	if config.Dgraph.MaxRetries != 5 {
		t.Errorf("Expected default max retries 5, got %d", config.Dgraph.MaxRetries)
	}

	if config.NVD.MaxRetries != 20 {
		t.Errorf("Expected default NVD max retries 20, got %d", config.NVD.MaxRetries)
	}

	if config.Trivy.OutputDir != "outputs" {
		t.Errorf("Expected default Trivy output dir 'outputs', got %s", config.Trivy.OutputDir)
	}
}

func TestLoad_FromEnvironment(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("SERVER_PORT", "8080")
	_ = os.Setenv("DGRAPH_HOST", "localhost")
	_ = os.Setenv("DGRAPH_PORT", "9999")
	_ = os.Setenv("DGRAPH_MAX_RETRIES", "10")
	_ = os.Setenv("NVD_MAX_RETRIES", "30")
	_ = os.Setenv("TRIVY_OUTPUT_DIR", "/tmp/trivy")

	defer clearEnvVars()

	config, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Server.Port != 8080 {
		t.Errorf("Expected server port 8080, got %d", config.Server.Port)
	}

	if config.Dgraph.Host != "localhost" {
		t.Errorf("Expected Dgraph host 'localhost', got %s", config.Dgraph.Host)
	}

	if config.Dgraph.Port != 9999 {
		t.Errorf("Expected Dgraph port 9999, got %d", config.Dgraph.Port)
	}

	if config.Dgraph.MaxRetries != 10 {
		t.Errorf("Expected max retries 10, got %d", config.Dgraph.MaxRetries)
	}

	if config.NVD.MaxRetries != 30 {
		t.Errorf("Expected NVD max retries 30, got %d", config.NVD.MaxRetries)
	}

	if config.Trivy.OutputDir != "/tmp/trivy" {
		t.Errorf("Expected Trivy output dir '/tmp/trivy', got %s", config.Trivy.OutputDir)
	}
}

func TestLoad_InvalidIntegerUsesDefault(t *testing.T) {
	_ = os.Setenv("SERVER_PORT", "invalid")
	defer clearEnvVars()

	config, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should fall back to default
	if config.Server.Port != 3000 {
		t.Errorf("Expected default port 3000 for invalid value, got %d", config.Server.Port)
	}
}

func TestGetDgraphAddress(t *testing.T) {
	config := &DgraphConfig{
		Host: "alpha",
		Port: 9080,
	}

	address := config.GetDgraphAddress()
	expected := "alpha:9080"

	if address != expected {
		t.Errorf("Expected address %s, got %s", expected, address)
	}
}

func TestGetServerAddress(t *testing.T) {
	config := &ServerConfig{
		Port: 3000,
	}

	address := config.GetServerAddress()
	expected := ":3000"

	if address != expected {
		t.Errorf("Expected address %s, got %s", expected, address)
	}
}

func TestGetRatelBaseURL(t *testing.T) {
	config := &DgraphConfig{
		RatelUIPort: 8000,
	}

	url := config.GetRatelBaseURL()
	expected := "http://localhost:8000/?query="

	if url != expected {
		t.Errorf("Expected URL %s, got %s", expected, url)
	}
}

func clearEnvVars() {
	_ = os.Unsetenv("SERVER_PORT")
	_ = os.Unsetenv("DGRAPH_HOST")
	_ = os.Unsetenv("DGRAPH_PORT")
	_ = os.Unsetenv("DGRAPH_MAX_RETRIES")
	_ = os.Unsetenv("DGRAPH_RETRY_DELAY_MS")
	_ = os.Unsetenv("DGRAPH_MAX_TRAVERSAL_DEPTH")
	_ = os.Unsetenv("NVD_MAX_RETRIES")
	_ = os.Unsetenv("NVD_INITIAL_RETRY_DELAY_MS")
	_ = os.Unsetenv("TRIVY_OUTPUT_DIR")
}
