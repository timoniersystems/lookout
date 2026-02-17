package logging

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithOutput(InfoLevel, &buf)

	// Debug should not appear (below threshold)
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Error("Debug message should not be logged at INFO level")
	}

	// Info should appear
	buf.Reset()
	logger.Info("info message")
	if !strings.Contains(buf.String(), "INFO") || !strings.Contains(buf.String(), "info message") {
		t.Error("Info message should be logged")
	}

	// Warn should appear
	buf.Reset()
	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "WARN") || !strings.Contains(buf.String(), "warn message") {
		t.Error("Warn message should be logged")
	}

	// Error should appear
	buf.Reset()
	logger.Error("error message")
	if !strings.Contains(buf.String(), "ERROR") || !strings.Contains(buf.String(), "error message") {
		t.Error("Error message should be logged")
	}
}

func TestLoggerSetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithOutput(InfoLevel, &buf)

	// Info should appear initially
	logger.Info("info message")
	if buf.Len() == 0 {
		t.Error("Info message should be logged at INFO level")
	}

	// Change to ERROR level
	buf.Reset()
	logger.SetLevel(ErrorLevel)

	// Info should not appear now
	logger.Info("info message")
	if buf.Len() > 0 {
		t.Error("Info message should not be logged at ERROR level")
	}

	// Error should still appear
	buf.Reset()
	logger.Error("error message")
	if buf.Len() == 0 {
		t.Error("Error message should be logged at ERROR level")
	}
}

func TestLoggerFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithOutput(InfoLevel, &buf)

	logger.Info("formatted %s %d", "message", 42)

	output := buf.String()
	if !strings.Contains(output, "formatted message 42") {
		t.Errorf("Expected formatted message, got: %s", output)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", DebugLevel},
		{"debug", DebugLevel},
		{"INFO", InfoLevel},
		{"info", InfoLevel},
		{"WARN", WarnLevel},
		{"WARNING", WarnLevel},
		{"ERROR", ErrorLevel},
		{"error", ErrorLevel},
		{"INVALID", InfoLevel}, // Should default to INFO
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLevel(tt.input); got != tt.expected {
				t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestInitFromEnv(t *testing.T) {
	// Save and restore original env
	originalLevel := os.Getenv("LOG_LEVEL")
	defer func() { _ = os.Setenv("LOG_LEVEL", originalLevel) }()

	// Test with DEBUG level
	_ = os.Setenv("LOG_LEVEL", "DEBUG")

	var buf bytes.Buffer
	defaultLogger.SetOutput(&buf)

	InitFromEnv()

	// Should log the initialization message
	if !strings.Contains(buf.String(), "Logger initialized") {
		t.Error("Expected initialization message")
	}

	// Verify DEBUG level is set
	buf.Reset()
	Debug("test debug")
	if buf.Len() == 0 {
		t.Error("Debug message should be logged after InitFromEnv with DEBUG")
	}
}

func TestGlobalLoggerFunctions(t *testing.T) {
	var buf bytes.Buffer
	SetGlobalOutput(&buf)
	SetGlobalLevel(DebugLevel)

	Debug("debug")
	if !strings.Contains(buf.String(), "DEBUG") {
		t.Error("Global Debug() should work")
	}

	buf.Reset()
	Info("info")
	if !strings.Contains(buf.String(), "INFO") {
		t.Error("Global Info() should work")
	}

	buf.Reset()
	Warn("warn")
	if !strings.Contains(buf.String(), "WARN") {
		t.Error("Global Warn() should work")
	}

	buf.Reset()
	Error("error")
	if !strings.Contains(buf.String(), "ERROR") {
		t.Error("Global Error() should work")
	}

	// Reset to defaults
	SetGlobalOutput(os.Stdout)
	SetGlobalLevel(InfoLevel)
}
