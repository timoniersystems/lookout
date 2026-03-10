package fileutil

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestCreateTempFromFormFile(t *testing.T) {
	// Create a test Echo context with a form file
	e := echo.New()

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	testContent := "test file content"
	_, _ = part.Write([]byte(testContent))
	_ = writer.Close()

	// Create HTTP request
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test the function
	result, err := CreateTempFromFormFile(c, "file")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify temp file exists
	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Errorf("Temp file does not exist: %s", result.Path)
	}

	// Verify content
	content, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content %q, got %q", testContent, string(content))
	}

	// Test cleanup
	if err := result.Cleanup(); err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(result.Path); !os.IsNotExist(err) {
		t.Errorf("Temp file still exists after cleanup: %s", result.Path)
	}
}

func TestCreateTempFromFormFile_MissingFile(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_, err := CreateTempFromFormFile(c, "file")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestCreateTempFromMultipartFile(t *testing.T) {
	// Create a test multipart file
	content := "test content for multipart"

	// Create a multipart file manually
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	_, _ = part.Write([]byte(content))
	_ = writer.Close()

	// Parse the multipart form
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, fileHeader, err := req.FormFile("file")
	if err != nil {
		t.Fatalf("Failed to get form file: %v", err)
	}

	file, err := fileHeader.Open()
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Test the function
	result, err := CreateTempFromMultipartFile(file, "test.txt")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify temp file exists
	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Errorf("Temp file does not exist: %s", result.Path)
	}

	// Verify content
	fileContent, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if string(fileContent) != content {
		t.Errorf("Expected content %q, got %q", content, string(fileContent))
	}

	// Test cleanup
	if err := result.Cleanup(); err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(result.Path); !os.IsNotExist(err) {
		t.Errorf("Temp file still exists after cleanup: %s", result.Path)
	}
}
