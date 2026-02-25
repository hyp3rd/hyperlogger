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

	defer func() {
		require.NoError(t, writer.Close())
	}()

	// Write test data
	testData := []byte("test log data for compression")
	err = os.WriteFile(logPath, testData, 0o600)
	require.NoError(t, err)

	// Trigger compression
	writer.compressFile(logPath)

	// Check compressed file exists
	compressedPath := logPath + ".gz"
	_, err = os.Stat(compressedPath)
	require.NoError(t, err)

	// Verify original file is removed
	_, err = os.Stat(logPath)
	assert.True(t, os.IsNotExist(err))

	// Read and verify compressed content
	// #nosec G304 -- test reads a file created in a temporary directory
	compressedFile, err := os.Open(compressedPath)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, compressedFile.Close())
	}()

	gzReader, err := gzip.NewReader(compressedFile)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, gzReader.Close())
	}()

	decompressed, err := io.ReadAll(gzReader)
	require.NoError(t, err)
	assert.Equal(t, testData, decompressed)
}

func TestCopyWithBuffer(t *testing.T) {
	const (
		defaultBufferSize = 32
		largeInputSize    = 100
	)

	tests := []struct {
		name        string
		input       []byte
		bufferSize  int
		expectError bool
	}{
		{
			name:       "copy empty data",
			input:      make([]byte, 0), // Explicitly create empty slice
			bufferSize: defaultBufferSize,
		},
		{
			name:       "copy data larger than buffer",
			input:      bytes.Repeat([]byte("a"), largeInputSize),
			bufferSize: defaultBufferSize,
		},
		{
			name:       "copy empty data",
			input:      []byte{},
			bufferSize: defaultBufferSize,
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
				require.NoError(t, err)
				assert.Equal(t, tt.input, dst.Bytes())
			}
		})
	}
}

func TestVerifyCompressedFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		setupFile   func(*testing.T, string) string
		expectError bool
	}{
		{
			name: "valid compressed file",
			setupFile: func(t *testing.T, dir string) string {
				t.Helper()

				path := filepath.Join(dir, "valid.gz")
				// #nosec G304 -- test creates a file in a temporary directory
				f, err := os.Create(path)
				require.NoError(t, err)

				gw := gzip.NewWriter(f)
				_, err = gw.Write([]byte("test data"))
				require.NoError(t, err)
				require.NoError(t, gw.Close())
				require.NoError(t, f.Close())

				return path
			},
			expectError: false,
		},
		{
			name: "empty file",
			setupFile: func(t *testing.T, dir string) string {
				t.Helper()

				path := filepath.Join(dir, "empty.gz")
				// #nosec G304 -- test creates a file in a temporary directory
				f, err := os.Create(path)
				require.NoError(t, err)
				require.NoError(t, f.Close())

				return path
			},
			expectError: true,
		},
		{
			name: "invalid gzip file",
			setupFile: func(t *testing.T, dir string) string {
				t.Helper()

				path := filepath.Join(dir, "invalid.gz")
				err := os.WriteFile(path, []byte("not a gzip file"), 0o600)
				require.NoError(t, err)

				return path
			},
			expectError: true,
		},
		{
			name: "non-existent file",
			setupFile: func(_ *testing.T, dir string) string {
				return filepath.Join(dir, "nonexistent.gz")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFile(t, tempDir)

			err := verifyCompressedFile(path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
