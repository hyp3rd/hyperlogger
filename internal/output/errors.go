package output

import (
	"github.com/hyp3rd/ewrap"
)

// Common errors for the output package.
var (
	// ErrWriterClosed is returned when attempting to write to a closed writer.
	ErrWriterClosed = ewrap.New("writer is closed")

	// ErrBufferFull is returned when the async writer's buffer is full.
	ErrBufferFull = ewrap.New("write buffer is full")

	// ErrFlushTimeout is returned when a flush operation times out.
	ErrFlushTimeout = ewrap.New("flush timed out")

	// ErrInvalidCompression is returned when an invalid compression algorithm is selected.
	ErrInvalidCompression = ewrap.New("invalid compression algorithm")

	// ErrCompressionFailed is returned when a compression operation fails.
	ErrCompressionFailed = ewrap.New("compression failed")
)
