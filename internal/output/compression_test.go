package output

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileWriter_CompressFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	writer, err := NewFileWriter(FileConfig{
		Path:     logPath,
		MaxSize:  100,
		Compress: true,
	})
	require.NoError(t, err)

	defer writer.Close()

	// Write test data
	testData := []byte("test log data for compression")
	err = os.WriteFile(logPath, testData, 0o644)
	require.NoError(t, err)

	// Trigger compression
	writer.compressFile(logPath)

	// Check compressed file exists
	compressedPath := logPath + ".gz"
	_, err = os.Stat(compressedPath)
	assert.NoError(t, err)

	// Verify original file is removed
	_, err = os.Stat(logPath)
	assert.True(t, os.IsNotExist(err))

	// Read and verify compressed content
	compressedFile, err := os.Open(compressedPath)
	require.NoError(t, err)

	defer compressedFile.Close()

	gzReader, err := gzip.NewReader(compressedFile)
	require.NoError(t, err)

	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	require.NoError(t, err)
	assert.Equal(t, testData, decompressed)
}

func TestCopyWithBuffer(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		bufferSize  int
		expectError bool
	}{
		{
			name:       "copy empty data",
			input:      make([]byte, 0), // Explicitly create empty slice
			bufferSize: 32,
		},
		{
			name:       "copy data larger than buffer",
			input:      bytes.Repeat([]byte("a"), 100),
			bufferSize: 32,
		},
		{
			name:       "copy empty data",
			input:      []byte{},
			bufferSize: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := bytes.NewReader(tt.input)
			dst := bytes.NewBuffer(make([]byte, 0)) // Initialize with empty slice
			buf := make([]byte, tt.bufferSize)

			err := copyWithBuffer(dst, src, buf)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.input, dst.Bytes())
			}
		})
	}
}

func TestVerifyCompressedFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		setupFile   func(string) string
		expectError bool
	}{
		{
			name: "valid compressed file",
			setupFile: func(dir string) string {
				path := filepath.Join(dir, "valid.gz")
				f, _ := os.Create(path)
				gw := gzip.NewWriter(f)
				gw.Write([]byte("test data"))
				gw.Close()
				f.Close()

				return path
			},
			expectError: false,
		},
		{
			name: "empty file",
			setupFile: func(dir string) string {
				path := filepath.Join(dir, "empty.gz")
				f, _ := os.Create(path)
				f.Close()

				return path
			},
			expectError: true,
		},
		{
			name: "invalid gzip file",
			setupFile: func(dir string) string {
				path := filepath.Join(dir, "invalid.gz")
				os.WriteFile(path, []byte("not a gzip file"), 0o644)

				return path
			},
			expectError: true,
		},
		{
			name: "non-existent file",
			setupFile: func(dir string) string {
				return filepath.Join(dir, "nonexistent.gz")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFile(tempDir)

			err := verifyCompressedFile(path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
