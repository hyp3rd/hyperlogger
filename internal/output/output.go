// Package output provides flexible output destinations for the logger package.
//
// This package implements various writers for log output destinations, including:
// - Console output with automatic color support based on terminal capabilities
// - File output with automatic rotation and compression
// - MultiWriter for sending logs to multiple destinations simultaneously
//
// Each writer implements the Writer interface, which extends io.Writer with methods
// for synchronization and cleanup:
//
//	type Writer interface {
//	    io.Writer
//	    Sync() error  // Ensures all data is written
//	    Close() error // Releases resources
//	}
//
// FileWriter provides file-based logging with:
// - Automatic log rotation based on file size
// - Optional compression of rotated log files
// - Safe concurrent access
// - Proper file cleanup and error handling
//
// ConsoleWriter provides console output with:
// - Automatic color detection for terminals
// - ANSI color support based on log level
// - Custom styling options
//
// MultiWriter combines multiple output destinations:
// - Thread-safe concurrent access
// - Detailed diagnostics for write failures
// - Dynamic addition/removal of writers
package output

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hyp3rd/ewrap"
	"github.com/mattn/go-isatty"

	"github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/internal/utils"
)

const (
	defaultMaxSizeMB  = 100
	defaultBufferSize = 4096 // 4KB buffer size
	bytesPerMB        = 1024 * 1024
	maxLookupBytes    = 32
)

// FileWriter implements Writer for file-based logging. It manages the log file, including
// rotating the file when it reaches a maximum size, and optionally compressing rotated files.
type FileWriter struct {
	mu                sync.Mutex
	file              *os.File
	path              string
	maxSize           int64
	size              int64
	compress          bool
	compressionConfig CompressionConfig
	rotationCallback  func(string) // Called after rotation with the path of the rotated file
	errorHandler      func(error)  // Called when errors occur during file operations
}

// FileConfig holds configuration for file output.
//
// - Path is the log file path
// - MaxSize is the maximum size in bytes before rotation
// - Compress determines if rotated files should be compressed
// - FileMode sets the permissions for new log files.
// - CompressionConfig provides detailed compression options
// - RotationCallback is called after rotation with the path of the rotated file
// - ErrorHandler is called when errors occur during file operations.
type FileConfig struct {
	// Path is the log file path
	Path string
	// MaxSize is the maximum size in bytes before rotation
	MaxSize int64
	// Compress determines if rotated files should be compressed
	Compress bool
	// FileMode sets the permissions for new log files
	FileMode os.FileMode
	// CompressionConfig provides detailed compression options
	CompressionConfig *CompressionConfig
	// RotationCallback is called after rotation with the path of the rotated file
	RotationCallback func(string)
	// ErrorHandler is called when errors occur during file operations
	ErrorHandler func(error)
}

// NewFileWriter creates a new file-based log writer. It initializes a FileWriter
// instance based on the provided FileConfig, ensuring the necessary directories
// and files are created. The returned FileWriter can be used to write log data
// to a file, with automatic rotation and compression of rotated files.
func NewFileWriter(config FileConfig) (*FileWriter, error) {
	if config.Path == "" {
		return nil, ewrap.New("log file path is required")
	}

	// Ensure path is secure
	securePath, err := utils.SecurePath(config.Path)
	if err != nil {
		return nil, ewrap.Wrap(err, "invalid log file path")
	}

	config.Path = securePath

	if config.MaxSize == 0 {
		config.MaxSize = defaultMaxSizeMB * bytesPerMB // 100MB default
	}

	if config.FileMode == 0 {
		config.FileMode = 0o644
	}

	// Setup compression config
	compressionConfig := defaultCompressionConfig()
	if config.CompressionConfig != nil {
		compressionConfig = *config.CompressionConfig
	}

	// Ensure directory exists
	dir := filepath.Dir(config.Path)

	err = os.MkdirAll(dir, 0o700)
	if err != nil {
		return nil, ewrap.Wrapf(err, "creating log directory").
			WithMetadata("path", dir)
	}

	// Open or create the log file
	file, err := os.OpenFile(config.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, config.FileMode)
	if err != nil {
		return nil, ewrap.Wrapf(err, "opening log file").
			WithMetadata("path", config.Path)
	}

	// Get initial file size
	info, err := file.Stat()
	if err != nil {
		ioErr := file.Close()
		if ioErr != nil {
			return nil, ewrap.Wrapf(ioErr, "closing file").
				WithMetadata("path", config.Path).
				WithMetadata("file", file).
				WithMetadata("err", err)
		}

		return nil, ewrap.Wrapf(err, "getting file stats").
			WithMetadata("path", config.Path)
	}

	return &FileWriter{
		file:              file,
		path:              config.Path,
		maxSize:           config.MaxSize,
		size:              info.Size(),
		compress:          config.Compress,
		compressionConfig: compressionConfig,
		rotationCallback:  config.RotationCallback,
		errorHandler:      config.ErrorHandler,
	}, nil
}

// Write implements io.Writer.
// It writes the provided data to the log file, handling automatic rotation
// of the log file when the maximum size is reached. It returns the number of
// bytes written and any error that occurred during the write operation.
func (w *FileWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if rotation is needed
	if w.size+int64(len(data)) > w.maxSize {
		err := w.rotate()
		if err != nil {
			if w.errorHandler != nil {
				w.errorHandler(err)
			}

			return 0, ewrap.Wrapf(err, "rotating log file")
		}
	}

	bytesWritten, err := w.file.Write(data)
	if err != nil {
		if w.errorHandler != nil {
			w.errorHandler(err)
		}

		return bytesWritten, ewrap.Wrap(err, "failed writing to log file")
	}

	w.size += int64(bytesWritten)

	return bytesWritten, nil // Return nil error on success, don't wrap it
}

// Sync ensures any buffered data is written to the underlying file.
// If the file has already been closed, Sync returns nil without error.
// Otherwise, it returns an error if the file could not be synced.
func (w *FileWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil // Already closed, no error
	}

	err := w.file.Sync()
	if err != nil {
		return ewrap.Wrapf(err, "syncing log file")
	}

	return nil // Clean success
}

// Close closes the underlying file. It first syncs any remaining data to the file,
// and then closes the file. If the file has already been closed, Close returns
// nil without error.
func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil // Already closed, no error
	}

	// First sync any remaining data
	err := w.file.Sync()
	if err != nil {
		return ewrap.Wrapf(err, "final sync before close")
	}

	// Then close the file
	err = w.file.Close()
	if err != nil {
		return ewrap.Wrapf(err, "closing log file")
	}

	w.file = nil // Mark as closed

	return nil // Clean success
}

// ColorMode determines how colors are handled.
type ColorMode int

const (
	// ColorModeAuto detects if the output supports colors.
	ColorModeAuto ColorMode = iota
	// ColorModeAlways forces color output.
	ColorModeAlways
	// ColorModeNever disables color output.
	ColorModeNever
)

// ConsoleWriter is a writer that writes to the console with color support and performance optimizations.
type ConsoleWriter struct {
	out        io.Writer
	mode       ColorMode
	isTerminal bool
	buffer     *bytes.Buffer
	mu         sync.Mutex
	style      map[hyperlogger.Level]struct {
		color ColorCode
		style Style
	}
}

// NewConsoleWriter creates a new ConsoleWriter with improved initialization and performance.
// The ConsoleWriter is a writer that writes to the console with color support and performance optimizations.
// It takes an io.Writer and a ColorMode as input, and returns a new ConsoleWriter instance.
// If the provided io.Writer is nil, it defaults to os.Stdout.
// The ConsoleWriter initializes its internal state, including the output mode, terminal detection,
// and color styles for different log levels.
func NewConsoleWriter(out io.Writer, mode ColorMode) *ConsoleWriter {
	if out == nil {
		out = os.Stdout
	}

	writer := &ConsoleWriter{
		out:        out,
		mode:       mode,
		isTerminal: IsTerminal(out),
		buffer:     bytes.NewBuffer(make([]byte, 0, defaultBufferSize)),
		style: map[hyperlogger.Level]struct {
			color ColorCode
			style Style
		}{
			hyperlogger.TraceLevel: {ColorWhite, StyleDim},
			hyperlogger.DebugLevel: {ColorCyan, StyleNormal},
			hyperlogger.InfoLevel:  {ColorGreen, StyleNormal},
			hyperlogger.WarnLevel:  {ColorYellow, StyleBold},
			hyperlogger.ErrorLevel: {ColorRed, StyleBold},
			hyperlogger.FatalLevel: {ColorMagenta, StyleBold},
		},
	}

	return writer
}

// Write implements the io.Writer interface for the ConsoleWriter. It writes the provided payload to the console
// with color support and performance optimizations. It first locks the writer's mutex, then resets and resizes the
// internal buffer as needed. It then checks if color output should be used based on the configured color mode and
// the detected log level of the payload. If colors should be used, it applies the appropriate ANSI escape codes
// before writing the payload to the underlying io.Writer. Finally, it returns the number of bytes written and any
// error that occurred during the write operation.
func (w *ConsoleWriter) Write(payload []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Reset buffer and ensure it has enough capacity
	w.buffer.Reset()

	if w.buffer.Cap() < len(payload)*2 { // Allow room for color codes
		w.buffer.Grow(len(payload) * 2)
	}

	// Check if colors should be applied
	if w.shouldUseColors() {
		level := w.detectLevel(payload) // Detect log level from content
		if style, ok := w.style[level]; ok {
			w.applyStyle(style.color, style.style)
			defer w.resetStyle()
		}
	}

	bytesWritten, err := w.out.Write(payload)
	if err != nil {
		return 0, ewrap.Wrap(err, "failed writing to console output")
	}

	return bytesWritten, nil
}

// Sync synchronizes the underlying io.Writer if it implements the Sync() error interface.
// If the underlying writer does not implement Sync(), this method returns nil.
func (w *ConsoleWriter) Sync() error {
	// For stdout/stderr, we can safely return nil
	if f, ok := w.out.(*os.File); ok {
		if f == os.Stdout || f == os.Stderr {
			return nil
		}
	}

	if syncer, ok := w.out.(interface{ Sync() error }); ok {
		return syncer.Sync()
	}

	return nil
}

// Close closes the underlying io.Writer if it implements the io.Closer interface.
// If an error occurs while closing the underlying writer, it is wrapped and returned.
func (w *ConsoleWriter) Close() error {
	if closer, ok := w.out.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return ewrap.Wrap(err, "closing console writer")
		}
	}

	return nil
}

// rotate moves the current log file to a timestamped backup
// and creates a new log file.
func (w *FileWriter) rotate() error {
	// Close current file
	err := w.file.Close()
	if err != nil {
		return ewrap.Wrapf(err, "closing current log file")
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	backupPath := filepath.Join(
		filepath.Dir(w.path),
		fmt.Sprintf("%s.%s", filepath.Base(w.path), timestamp),
	)

	// Rename current file to backup
	err = os.Rename(w.path, backupPath)
	if err != nil {
		return ewrap.Wrapf(err, "renaming log file").
			WithMetadata("from", w.path).
			WithMetadata("to", backupPath)
	}

	// Compress backup file if enabled
	if w.compress {
		go w.compressFile(backupPath) // Run compression in background
	}

	// Create new log file

	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return ewrap.Wrapf(err, "creating new log file")
	}

	w.file = file
	w.size = 0

	return nil
}

// shouldUseColors determines if color output should be used based on mode and terminal support.
//
//nolint:exhaustive // ColorModeAuto is handled as default.
func (w *ConsoleWriter) shouldUseColors() bool {
	switch w.mode {
	case ColorModeAlways:
		return true
	case ColorModeNever:
		return false
	default: // ColorModeAuto
		return w.isTerminal
	}
}

// detectLevel attempts to determine the log level from the content.
// This is a fast approximation that looks for level indicators in the first few bytes.
func (*ConsoleWriter) detectLevel(p []byte) hyperlogger.Level {
	// Look for common level indicators in the first 32 bytes
	head := p
	if len(p) > maxLookupBytes {
		head = p[:maxLookupBytes]
	}

	switch {
	case bytes.Contains(head, []byte("TRACE")):
		return hyperlogger.TraceLevel
	case bytes.Contains(head, []byte("DEBUG")):
		return hyperlogger.DebugLevel
	case bytes.Contains(head, []byte("INFO")):
		return hyperlogger.InfoLevel
	case bytes.Contains(head, []byte("WARN")):
		return hyperlogger.WarnLevel
	case bytes.Contains(head, []byte("ERROR")):
		return hyperlogger.ErrorLevel
	case bytes.Contains(head, []byte("FATAL")):
		return hyperlogger.FatalLevel
	default:
		return hyperlogger.InfoLevel
	}
}

// applyStyle writes the ANSI escape codes for the given color and style.
func (w *ConsoleWriter) applyStyle(color ColorCode, style Style) {
	if style != StyleNormal {
		fmt.Fprintf(w.buffer, "\x1b[%d;%dm", style, color)
	} else {
		fmt.Fprintf(w.buffer, "\x1b[%dm", color)
	}
}

// resetStyle writes the ANSI escape code to reset all formatting.
func (w *ConsoleWriter) resetStyle() {
	w.buffer.WriteString("\x1b[0m")
}

// MultiWriter combines multiple writers into one.
type MultiWriter struct {
	mu sync.RWMutex

	Writers []Writer
	// Add a debug name for each writer to help with diagnostics
	writerNames map[Writer]string
}

// NewMultiWriter creates a new writer that writes to all provided writers.
// It filters out nil writers and returns an error if no valid writers are provided.
func NewMultiWriter(writers ...Writer) (*MultiWriter, error) {
	if len(writers) == 0 {
		return nil, ewrap.New("at least one writer is required")
	}

	validWriters := make([]Writer, 0, len(writers))
	writerNames := make(map[Writer]string)

	// Create descriptive names for each writer
	for i, w := range writers {
		if w != nil {
			validWriters = append(validWriters, w)
			// Store a descriptive name based on the writer type
			writerNames[w] = fmt.Sprintf("%T[%d]", w, i)
		}
	}

	if len(validWriters) == 0 {
		return nil, ewrap.New("no valid writers provided")
	}

	return &MultiWriter{
		Writers:     validWriters,
		writerNames: writerNames,
	}, nil
}

// Write sends the output to all writers with detailed diagnostics.
func (mw *MultiWriter) Write(payload []byte) (int, error) {
	mw.mu.RLock()
	defer mw.mu.RUnlock()

	return mw.writeToWriters(payload)
}

// Sync ensures all writers are synced with comprehensive diagnostics.
// It iterates through the list of writers in the MultiWriter, calling the Sync()
// method on each one. It tracks the number of successful and failed syncs,
// and returns an error if any of the syncs fail, including a detailed error
// report with the failure details.
func (mw *MultiWriter) Sync() error {
	mw.mu.RLock()
	defer mw.mu.RUnlock()

	var syncErrors []string

	for _, writer := range mw.Writers {
		if writer == nil || shouldBypassSync(writer) {
			continue
		}

		err := writer.Sync()
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("%T: %v", writer, err))
		}
	}

	if len(syncErrors) > 0 {
		return ewrap.New("sync operation partially failed").
			WithMetadata("failed_syncs", syncErrors).
			WithMetadata("total_writers", len(mw.Writers))
	}

	return nil
}

// Close closes all writers with detailed cleanup tracking.
// It iterates through the list of writers in the MultiWriter, calling the Close() method on each one.
// It tracks the number of successful and failed closes, and returns an error if any of the closes fail,
// including a detailed error report with the failure details.
func (mw *MultiWriter) Close() error {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	var closeErrors []string

	for _, writer := range mw.Writers {
		if writer == nil || shouldBypassClose(writer) {
			continue
		}

		err := writer.Close()
		if err != nil {
			closeErrors = append(closeErrors, fmt.Sprintf("%T: %v", writer, err))
		}
	}

	// Clear writers slice
	for i := range mw.Writers {
		mw.Writers[i] = nil
	}

	mw.Writers = nil

	if len(closeErrors) > 0 {
		return ewrap.New("close operation partially failed").
			WithMetadata("failed_closes", closeErrors).
			WithMetadata("total_writers", len(closeErrors))
	}

	return nil
}

// AddWriter adds a new writer to the MultiWriter. If the provided writer is nil,
// an error is returned.
func (mw *MultiWriter) AddWriter(writer Writer) error {
	if writer == nil {
		return ewrap.New("cannot add nil writer")
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	// Check for duplicates
	for _, existing := range mw.Writers {
		if existing == writer {
			return ewrap.New("writer already exists in MultiWriter")
		}
	}

	mw.Writers = append(mw.Writers, writer)
	mw.writerNames[writer] = fmt.Sprintf("%T[%d]", writer, len(mw.Writers)-1)

	return nil
}

// RemoveWriter removes a writer from the MultiWriter.
func (mw *MultiWriter) RemoveWriter(writer Writer) {
	if writer == nil {
		return
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	for i, existingWriter := range mw.Writers {
		if existingWriter == writer {
			// Remove the writer by replacing it with the last element
			// and truncating the slice
			lastIdx := len(mw.Writers) - 1
			mw.Writers[i] = mw.Writers[lastIdx]
			mw.Writers[lastIdx] = nil // Clear the reference
			mw.Writers = mw.Writers[:lastIdx]

			break
		}
	}
}

// writeToWriters writes the payload to all writers in the MultiWriter, tracking the results of each write.
// This method implements the core logic for the MultiWriter.Write method, handling write operations
// to multiple destinations with comprehensive error tracking.
func (mw *MultiWriter) writeToWriters(payload []byte) (int, error) {
	if len(mw.Writers) == 0 {
		return 0, nil
	}

	failedWrites, incompleteWrites, successCount := mw.writeToEachWriter(payload)

	return mw.prepareResult(payload, failedWrites, incompleteWrites, successCount)
}

// writeToEachWriter attempts to write the payload to each writer and tracks results.
//
//nolint:nonamedreturns
func (mw *MultiWriter) writeToEachWriter(
	payload []byte,
) (failedWrites []string, incompleteWrites []string, successCount int) {
	totalBytes := len(payload)

	for _, writer := range mw.Writers {
		if writer == nil {
			continue
		}

		writerName := mw.getWriterName(writer)
		bytesWritten, err := writer.Write(payload)

		switch {
		case err != nil:
			failedWrites = append(failedWrites, fmt.Sprintf("%s: %v", writerName, err))
		case bytesWritten != totalBytes:
			incompleteWrites = append(
				incompleteWrites,
				fmt.Sprintf("%s: wrote %d/%d bytes", writerName, bytesWritten, totalBytes),
			)
		default:
			successCount++
		}
	}

	return failedWrites, incompleteWrites, successCount
}

func shouldBypassSync(writer Writer) bool {
	switch w := writer.(type) {
	case *os.File:
		return isStandardStream(w)
	case *writerAdapter:
		if f, ok := w.writer.(*os.File); ok {
			return isStandardStream(f)
		}
	}

	return false
}

func shouldBypassClose(writer Writer) bool {
	switch w := writer.(type) {
	case *os.File:
		return isStandardStream(w)
	case *writerAdapter:
		if f, ok := w.writer.(*os.File); ok {
			return isStandardStream(f)
		}
	}

	return false
}

func isStandardStream(f *os.File) bool {
	return f == os.Stdout || f == os.Stderr
}

// getWriterName returns a descriptive name for a writer.
func (mw *MultiWriter) getWriterName(writer Writer) string {
	writerName, ok := mw.writerNames[writer]
	if !ok {
		writerName = fmt.Sprintf("%T", writer)
	}

	return writerName
}

// prepareResult determines the appropriate return values based on write results.
func (mw *MultiWriter) prepareResult(
	payload []byte,
	failedWrites []string,
	incompleteWrites []string,
	successCount int,
) (int, error) {
	totalBytes := len(payload)

	// Return success if all writers succeeded
	if len(failedWrites) == 0 && len(incompleteWrites) == 0 {
		return totalBytes, nil
	}

	// Create error message
	errMsg := mw.buildErrorMessage(failedWrites, incompleteWrites, successCount)

	// Return behavior depends on whether any writes succeeded
	if successCount > 0 {
		return totalBytes, ewrap.New(errMsg)
	}

	return 0, ewrap.New(errMsg)
}

// buildErrorMessage creates an error message from the results of write operations.
func (mw *MultiWriter) buildErrorMessage(failedWrites, incompleteWrites []string, successCount int) string {
	var errMsg strings.Builder

	errMsg.WriteString(fmt.Sprintf("write operation partially failed (%d/%d writers succeeded)",
		successCount, len(mw.Writers)))

	if len(failedWrites) > 0 {
		errMsg.WriteString("\nFailed writes:\n  ")
		errMsg.WriteString(strings.Join(failedWrites, "\n  "))
	}

	if len(incompleteWrites) > 0 {
		errMsg.WriteString("\nIncomplete writes:\n  ")
		errMsg.WriteString(strings.Join(incompleteWrites, "\n  "))
	}

	return errMsg.String()
}

// IsTerminal checks if the given writer is a terminal. It returns true if the writer is
// connected to a terminal, and false otherwise. This function is used to determine
// whether to enable color support for log output.
func IsTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		if f.Fd() == uintptr(syscall.Stdout) || f.Fd() == uintptr(syscall.Stderr) {
			return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
		}
	}

	return false
}
