// Package utils provides internal utility functions used throughout the logger package.
//
// This package contains helper functions for common tasks such as file path handling,
// security utilities, and I/O operations. These utilities are primarily for internal use
// by the logger package and are not intended to be part of the public API.
//
// The utilities include path security functions to prevent directory traversal attacks
// when handling user-supplied file paths.
package utils

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hyp3rd/ewrap"
)

// SecurePath takes a relative path string and returns a secure absolute path within
// the system's temporary directory. This function helps prevent path traversal attacks
// and unauthorized file access.
//
// The function performs several security checks, including:
// - Rejecting empty paths
// - Normalizing the path using filepath.Clean
// - Preventing directory traversal sequences (..)
// - Disallowing absolute paths (except those already within the system temp directory)
// - Verifying that any symlinks in the path don't escape the temporary directory
//
// Parameters:
//   - path: The input path to secure, expected to be a relative path
//
// Returns:
//   - A secured absolute path within the system's temporary directory
//   - An error if any security check fails or if the path is invalid
//
// Note: This function is designed to be used before performing file operations
// where user-supplied paths need to be confined to a safe directory.
func SecurePath(path string) (string, error) {
	// Check for empty path
	if path == "" {
		return "", ewrap.New("path cannot be empty")
	}

	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return "", ewrap.New("invalid path contains directory traversal sequence").
			WithMetadata("path", path)
	}

	// Check for absolute paths
	if filepath.IsAbs(cleanPath) {
		// Allow absolute paths in the temporary directory for tests
		tempDir := os.TempDir()
		if strings.HasPrefix(path, tempDir) {
			return path, nil
		}

		return "", ewrap.New("absolute paths are not allowed").WithMetadata("path", path)
	}

	// Get the system's temp directory instead of hardcoding "/tmp"
	tempDir := os.TempDir()

	// Construct the full path
	fullPath := filepath.Join(tempDir, cleanPath)

	// Handle symlinks - resolve and verify they don't escape the temp directory
	resolvedPath, err := filepath.EvalSymlinks(fullPath)
	if err == nil { // Only check if the path exists and can be resolved
		// Ensure the resolved path is still within the temp directory
		if !strings.HasPrefix(resolvedPath, tempDir) {
			return "", ewrap.New("path resolves to location outside of temp directory").
				WithMetadata("path", path)
		}
	}

	return fullPath, nil
}
