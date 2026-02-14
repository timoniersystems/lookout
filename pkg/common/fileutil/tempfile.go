// Package fileutil provides file handling utilities for temporary files and uploads.
package fileutil

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
)

// TempFileResult contains the path to a temporary file and a cleanup function
type TempFileResult struct {
	Path    string
	Cleanup func() error
}

// CreateTempFromFormFile creates a temporary file from an Echo form file upload.
// It returns the file path and a cleanup function that removes the temp file.
// The caller is responsible for calling the cleanup function when done.
func CreateTempFromFormFile(c echo.Context, formKey string) (*TempFileResult, error) {
	// Get the uploaded file
	fileHeader, err := c.FormFile(formKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve file from form: %w", err)
	}

	// Open the uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Create temporary file
	tempFile, err := os.CreateTemp("", "upload-*"+filepath.Ext(fileHeader.Filename))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Copy uploaded file to temp file
	if _, err := io.Copy(tempFile, src); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	// Close the temp file
	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Create cleanup function
	cleanup := func() error {
		return os.Remove(tempFile.Name())
	}

	return &TempFileResult{
		Path:    tempFile.Name(),
		Cleanup: cleanup,
	}, nil
}

// CreateTempFromMultipartFile creates a temporary file from a multipart file.
// This is useful when you already have a multipart.File object.
func CreateTempFromMultipartFile(file multipart.File, filename string) (*TempFileResult, error) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "upload-*"+filepath.Ext(filename))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Copy uploaded file to temp file
	if _, err := io.Copy(tempFile, file); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	// Close the temp file
	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Create cleanup function
	cleanup := func() error {
		return os.Remove(tempFile.Name())
	}

	return &TempFileResult{
		Path:    tempFile.Name(),
		Cleanup: cleanup,
	}, nil
}
