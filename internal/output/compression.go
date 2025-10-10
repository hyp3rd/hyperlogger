package output

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger/internal/utils"
)

// BufferSize represents different buffer sizes for memory optimization.
type BufferSize int

const (
	// SmallBuffer is optimized for small log entries.
	SmallBuffer BufferSize = 512
	// MediumBuffer is for average log entries.
	MediumBuffer BufferSize = 8 * 1024 // 8KB
	// LargeBuffer size for larger log entries.
	LargeBuffer BufferSize = 32 * 1024 // 32KB
	// XLBuffer for very large log entries or file operations.
	XLBuffer BufferSize = 64 * 1024 // 64KB
)

// CompressionLevel defines the compression level to use.
type CompressionLevel int

const (
	// CompressionDefault uses the default gzip compression level.
	CompressionDefault CompressionLevel = 1
	// CompressionBest uses the best (slowest but most effective) compression level.
	CompressionBest CompressionLevel = 9
	// CompressionSpeed optimizes for speed over compression ratio.
	CompressionSpeed CompressionLevel = 1
)

// GzipLevel converts the CompressionLevel to its corresponding gzip compression level integer.
// It allows seamless conversion between the custom CompressionLevel type and the standard gzip compression level.
func (c CompressionLevel) GzipLevel() int {
	return int(c)
}

// CompressionConfig configures compression for log files.
type CompressionConfig struct {
	// Algorithm is the compression algorithm to use.
	Algorithm CompressionAlgorithm
	// Level is the compression level to use.
	Level int
	// DeleteOriginal determines if the original file should be deleted after compression.
	DeleteOriginal bool
	// Extension is the file extension to use for compressed files (default: .gz).
	Extension string
}

// defaultCompressionConfig returns the default compression configuration.
func defaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Algorithm:      GzipCompression,
		Level:          gzip.DefaultCompression,
		DeleteOriginal: true,
		Extension:      ".gz",
	}
}

// compressFile compresses the given file using gzip compression.
// The original file is removed after successful compression.
// This method is designed to run in the background to avoid blocking logging operations.
func (w *FileWriter) compressFile(path string) {
	// We'll use a WaitGroup to ensure proper cleanup in case of panic
	var wg sync.WaitGroup

	wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				// If panic occurs, ensure we don't leave partial files
				cleanupCompression(path, w.compressionConfig.Extension)
			}
		}()

		err := w.performCompression(path)
		if err != nil {
			// Log the error but don't fail - this is a background operation
			// In a real application, you might want to send this to an error channel
			// or use your error reporting system
			_, err = os.Stderr.WriteString("Error compressing log file: " + err.Error() + "\n")
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to write error message: %v\n", err)
			}
		}
	})

	wg.Wait()
}

// performCompression handles the actual compression work.
// It compresses the file at the given path using gzip compression.
// The original file is removed after successful compression.
//
//nolint:cyclop,funlen,revive // 11 statements is acceptable for this function, a refactor would make it worse.
func (w *FileWriter) performCompression(path string) error {
	// Open the source file
	//nolint:gosec // G304: Potential file inclusion via variable: the path is already validated by SecurePath
	source, err := os.Open(path)
	if err != nil {
		return ewrap.Wrapf(err, "opening source file").
			WithMetadata("path", path)
	}

	defer func() {
		err := source.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close source file: %v\n", err)
		}
	}()

	// Create the compressed file
	compressedPath := path + w.compressionConfig.Extension
	//nolint:gosec // G304: Potential file inclusion via variable: the path is already validated by SecurePath
	compressed, err := os.OpenFile(compressedPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return ewrap.Wrapf(err, "creating compressed file").
			WithMetadata("path", compressedPath)
	}

	// Create a variable to track if we've explicitly closed the file
	var compressedClosed bool

	defer func() {
		// Only close in defer if not already explicitly closed
		if !compressedClosed && compressed != nil {
			err := compressed.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to close compressed file: %v\n", err)
			}
		}
	}()

	// Create gzip writer with appropriate compression level
	gzipLevel := w.getGzipCompressionLevel()

	gzipWriter, err := gzip.NewWriterLevel(compressed, gzipLevel)
	if err != nil {
		return ewrap.Wrapf(err, "creating gzip writer")
	}

	defer func() {
		err := gzipWriter.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close gzip writer: %v\n", err)
		}
	}()

	// Set the original file name in the gzip header
	gzipWriter.Name = filepath.Base(path)

	// Create a buffer for copying from the buffer pool
	buffer := getCompressionBuffer(LargeBuffer)
	defer putCompressionBuffer(buffer, LargeBuffer)

	// Copy the file content in chunks
	err = copyWithBuffer(gzipWriter, source, buffer)
	if err != nil {
		// If compression fails, clean up the partial compressed file
		return ewrap.Wrapf(handleCompressedPathRemoval(&compressedPath, err), "copying file content").
			WithMetadata("path", path).
			WithMetadata("compressed_path", compressedPath)
	}

	// Ensure all data is written
	err = gzipWriter.Close()
	if err != nil {
		return ewrap.Wrapf(handleCompressedPathRemoval(&compressedPath, err), "closing gzip writer").
			WithMetadata("path", path).
			WithMetadata("compressed_path", compressedPath)
	}

	err = compressed.Sync()
	if err != nil {
		return ewrap.Wrapf(handleCompressedPathRemoval(&compressedPath, err), "syncing compressed file").
			WithMetadata("path", path).
			WithMetadata("compressed_path", compressedPath)
	}

	// Close the file explicitly and mark it as closed
	err = compressed.Close()
	if err != nil {
		return ewrap.Wrapf(handleCompressedPathRemoval(&compressedPath, err),
			"closing compressed file").
			WithMetadata("path", path).
			WithMetadata("compressed_path", compressedPath)
	}

	compressedClosed = true

	// Verify the compressed file exists and has content
	err = verifyCompressedFile(compressedPath)
	if err != nil {
		return ewrap.Wrapf(handleCompressedPathRemoval(&compressedPath, err), "verifying compressed file").
			WithMetadata("path", path).
			WithMetadata("compressed_path", compressedPath)
	}

	// Remove the original file only after successful compression if configured to do so
	if w.compressionConfig.DeleteOriginal {
		err := os.Remove(path)
		if err != nil {
			// If we can't remove the original, remove the compressed file to avoid duplicates
			return ewrap.Wrapf(handleCompressedPathRemoval(&compressedPath, err), "removing original file").
				WithMetadata("path", path).
				WithMetadata("err", err)
		}
	}

	return nil
}

// getGzipCompressionLevel converts the FileWriter compression level to gzip level.
func (w *FileWriter) getGzipCompressionLevel() int {
	switch w.compressionConfig.Level {
	case CompressionBest.GzipLevel():
		return gzip.BestCompression
	case CompressionSpeed.GzipLevel():
		return gzip.BestSpeed
	case CompressionDefault.GzipLevel():
		return gzip.DefaultCompression
	default:
		return gzip.NoCompression
	}
}

// A pool for compression buffers.
//
//nolint:gochecknoglobals // Global variable for the compression buffer pool.
var compressionBufferPools = map[BufferSize]*sync.Pool{
	SmallBuffer: {
		New: func() any {
			return make([]byte, SmallBuffer)
		},
	},
	MediumBuffer: {
		New: func() any {
			return make([]byte, MediumBuffer)
		},
	},
	LargeBuffer: {
		New: func() any {
			return make([]byte, LargeBuffer)
		},
	},
	XLBuffer: {
		New: func() any {
			return make([]byte, XLBuffer)
		},
	},
}

// getCompressionBuffer gets a buffer from the pool for a specific size.
func getCompressionBuffer(size BufferSize) []byte {
	pool, ok := compressionBufferPools[size]
	if !ok {
		// If no pool exists for this size, create a new buffer
		return make([]byte, size)
	}

	buf, ok := pool.Get().([]byte)
	if !ok || len(buf) != int(size) {
		// If type assertion fails or buffer size doesn't match, create a new buffer
		return make([]byte, size)
	}

	return buf
}

// putCompressionBuffer returns a buffer to the pool.
func putCompressionBuffer(buffer []byte, size BufferSize) {
	if buffer == nil {
		return
	}

	// Only return the buffer to the pool if it's the right size
	if len(buffer) == int(size) {
		if pool, ok := compressionBufferPools[size]; ok {
			pool.Put(&buffer)
		}
	}
}

// handleCompressedPathRemoval attempts to remove a compressed file path if it exists,
// returning the original error regardless of success or failure in removing the file.
func handleCompressedPathRemoval(compressedPath *string, originalError error) error {
	if compressedPath != nil && *compressedPath != "" {
		_, err := os.Stat(*compressedPath)
		if err == nil {
			errRemove := os.Remove(*compressedPath)
			if errRemove != nil {
				// We still want to return the original error, but log this cleanup failure
				fmt.Fprintf(os.Stderr, "failed to remove partial compressed file %s: %v\n", *compressedPath, errRemove)
			}

			fmt.Fprintf(os.Stderr, "failed to remove partial compressed file %s: %v\n", *compressedPath, err)
		}
	}

	return originalError
}

// copyWithBuffer efficiently copies data from source to destination using a buffer.
// This is a specialized version optimized for log file compression with better
// performance than io.Copy for our specific use case.
func copyWithBuffer(dst io.Writer, src io.Reader, buffer []byte) error {
	if len(buffer) == 0 {
		return ewrap.New("zero-length buffer provided")
	}

	for {
		nr, err := src.Read(buffer)
		if nr > 0 {
			nw, writeErr := dst.Write(buffer[:nr])
			if writeErr != nil {
				return ewrap.Wrapf(writeErr, "writing to destination")
			}

			if nw != nr {
				return ewrap.New("short write")
			}
		}

		if err != nil {
			if err == io.EOF {
				return nil // Normal completion
			}

			return ewrap.Wrapf(err, "reading from source")
		}
	}
}

// verifyCompressedFile verifies that a compressed file is valid.
func verifyCompressedFile(path string) error {
	// Open the compressed file
	//nolint:gosec // G304: Potential file inclusion via variable: the path is already validated by SecurePath
	file, err := os.Open(path)
	if err != nil {
		return ewrap.Wrapf(err, "opening compressed file for verification")
	}

	defer func() {
		err := file.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close file during verification: %v\n", err)
		}
	}()

	// Check if file exists and has content
	info, err := file.Stat()
	if err != nil {
		return ewrap.Wrapf(err, "getting file stats for verification")
	}

	if info.Size() == 0 {
		return ewrap.New("compressed file is empty")
	}

	// Create a gzip reader and verify header
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return ewrap.Wrapf(err, "creating gzip reader for verification")
	}

	defer func() {
		err = gzipReader.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close gzip reader: %v\n", err)
		}
	}()

	// Read a small amount of content to verify it's valid gzip
	buffer := make([]byte, 1024)

	_, err = gzipReader.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return ewrap.Wrapf(err, "verifying gzip content")
	}

	return nil
}

// cleanupCompression removes any partial files after a failed compression.
func cleanupCompression(originalPath, extension string) {
	compressedPath := originalPath + extension

	_, err := os.Stat(compressedPath)
	if err == nil {
		// If compressed file exists, remove it
		err := os.Remove(compressedPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean up compressed file %s: %v\n", compressedPath, err)
		}
	}
}

// CompressionAlgorithm represents a compression algorithm.
type CompressionAlgorithm string

const (
	// NoCompression represents no compression.
	NoCompression CompressionAlgorithm = "none"
	// GzipCompression represents gzip compression.
	GzipCompression CompressionAlgorithm = "gzip"
)

// CompressFile compresses a file using the specified algorithm and level.
// It takes the path to the file to compress, compresses the file,
// and saves it with the specified extension.
func CompressFile(path string, config CompressionConfig) (string, error) {
	switch config.Algorithm {
	case NoCompression:
		return path, nil
	case GzipCompression:
		return compressGzip(path, config.Level)
	default:
		return "", ErrInvalidCompression
	}
}

// compressGzip compresses a file using gzip compression.
func compressGzip(path string, level int) (string, error) {
	// Open the source file
	secPath, err := utils.SecurePath(path)
	if err != nil {
		return "", err
	}

	//nolint:gosec // G304: Potential file inclusion via variable: the path is already validated by SecurePath
	srcFile, err := os.Open(secPath)
	if err != nil {
		return "", ewrap.Wrap(err, "failed to open source file")
	}

	defer func() {
		err := srcFile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close source file: %v\n", err)
		}
	}()

	// Create the destination file
	dstPath := secPath + ".gz"
	//nolint:gosec // G304: Potential file inclusion via variable: the path is already validated by SecurePath.
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return "", ewrap.Wrap(err, "failed to create destination file")
	}

	defer func() {
		err := dstFile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close destination file: %v\n", err)
		}
	}()

	// Create a gzip writer
	gzipWriter, err := gzip.NewWriterLevel(dstFile, level)
	if err != nil {
		return "", ewrap.Wrap(err, "failed to create gzip writer")
	}

	defer func() {
		err := gzipWriter.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close gzip writer: %v\n", err)
		}
	}()

	// Copy data from source to destination
	_, err = io.Copy(gzipWriter, srcFile)
	if err != nil {
		return "", ewrap.Wrap(err, "failed to copy data to gzip writer")
	}

	// Close the gzip writer to flush all data
	err = gzipWriter.Close()
	if err != nil {
		return "", ewrap.Wrap(err, "failed to close gzip writer")
	}

	// Delete the original file after successful compression
	err = os.Remove(path)
	if err != nil {
		// Just log the error, don't fail
		return dstPath, nil
	}

	return dstPath, nil
}
