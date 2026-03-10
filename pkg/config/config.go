// Package config provides configuration management for the Lookout application.
// It supports loading configuration from environment variables with sensible defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	Server ServerConfig
	Dgraph DgraphConfig
	NVD    NVDConfig
	Trivy  TrivyConfig
}

// ServerConfig holds web server configuration
type ServerConfig struct {
	Port int
}

// DgraphConfig holds Dgraph database configuration
type DgraphConfig struct {
	Host            string
	Port            int
	MaxRetries      int
	RetryDelayMS    int
	MaxTraversalDepth int
	RatelUIPort     int
}

// NVDConfig holds National Vulnerability Database API configuration
type NVDConfig struct {
	MaxRetries      int
	InitialRetryDelayMS int
}

// TrivyConfig holds Trivy scanner configuration
type TrivyConfig struct {
	OutputDir string
}

// Load reads configuration from environment variables with defaults
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port: getEnvAsInt("SERVER_PORT", 3000),
		},
		Dgraph: DgraphConfig{
			Host:              getEnv("DGRAPH_HOST", "alpha"),
			Port:              getEnvAsInt("DGRAPH_PORT", 9080),
			MaxRetries:        getEnvAsInt("DGRAPH_MAX_RETRIES", 5),
			RetryDelayMS:      getEnvAsInt("DGRAPH_RETRY_DELAY_MS", 2000),
			MaxTraversalDepth: getEnvAsInt("DGRAPH_MAX_TRAVERSAL_DEPTH", 10),
			RatelUIPort:       getEnvAsInt("DGRAPH_RATEL_PORT", 8000),
		},
		NVD: NVDConfig{
			MaxRetries:          getEnvAsInt("NVD_MAX_RETRIES", 20),
			InitialRetryDelayMS: getEnvAsInt("NVD_INITIAL_RETRY_DELAY_MS", 5000),
		},
		Trivy: TrivyConfig{
			OutputDir: getEnv("TRIVY_OUTPUT_DIR", "outputs"),
		},
	}

	return config, nil
}

// getEnv reads an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt reads an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		fmt.Printf("Warning: Invalid integer value for %s: %s, using default %d\n", key, valueStr, defaultValue)
		return defaultValue
	}

	return value
}

// GetDgraphAddress returns the full Dgraph address
func (c *DgraphConfig) GetDgraphAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetServerAddress returns the server bind address
func (c *ServerConfig) GetServerAddress() string {
	return fmt.Sprintf(":%d", c.Port)
}

// GetRatelBaseURL returns the Ratel UI base URL
func (c *DgraphConfig) GetRatelBaseURL() string {
	return fmt.Sprintf("http://localhost:%d/?query=", c.RatelUIPort)
}
