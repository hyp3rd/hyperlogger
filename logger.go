// Package hyperlogger defines a flexible, high-performance logging system for Go applications.
//
// This package provides a comprehensive logging abstraction that supports:
// - Multiple log levels (Trace, Debug, Info, Warn, Error, Fatal)
// - Structured logging with typed fields
// - Context-aware logging for propagating request metadata
// - Color-coded output for terminal readability
// - Error integration with optional stack traces
// - Multiple output formats (text and JSON)
// - Asynchronous logging with fallback to synchronous for critical messages
// - Log file rotation and compression
//
// The core Logger interface defined in this package serves as the foundation for all
// logging operations throughout applications using this library. Concrete implementations
// of this interface are provided by the adapter package.
//
// This separation of interface from implementation allows for flexibility in logging
// backends while maintaining a consistent API for application code.
//
// Basic usage:
//
//	// Create a logger with default configuration
//	log, err := adapter.NewAdapter(logger.DefaultConfig())
//	if err != nil {
//		panic(err)
//	}
//
//	// Log messages at different levels
//	log.Info("Application started")
//	log.WithField("user", "admin").Debug("User logged in")
//	log.WithError(err).Error("Operation failed")
//
// Always call Sync() before application exit to ensure all logs are written:
//
//	defer log.Sync()
package hyperlogger

import (
	"context"
)

// Level represents the severity of a log message.
type Level uint8

const (
	// TraceLevel represents verbose debugging information.
	TraceLevel Level = iota
	// DebugLevel represents debugging information.
	DebugLevel
	// InfoLevel represents general operational information.
	InfoLevel
	// WarnLevel represents warning messages.
	WarnLevel
	// ErrorLevel represents error messages.
	ErrorLevel
	// FatalLevel represents fatal error messages.
	FatalLevel
)

// String returns the string representation of a log level.
func (l Level) String() string {
	switch l {
	case TraceLevel:
		return "TRACE"
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// IsValid returns true if the given Level is a valid log level, and false otherwise.
func (l Level) IsValid() bool {
	return l >= TraceLevel && l <= FatalLevel
}

// Field represents a key-value pair in structured logging.
type Field struct {
	Key   string
	Value any
}

// Logger defines the interface for logging operations.
type Logger interface {
	// Log methods for different levels
	Trace(msg string)
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)

	// Formatted log methods
	FormattedLogger

	Methods
}

// Methods defines the interface for logging methods.
type Methods interface {
	// WithContext adds context information to the logger
	WithContext(ctx context.Context) Logger
	// WithFields adds structured fields to the logger
	WithFields(fields ...Field) Logger
	// WithField adds a single field to the logger
	WithField(key string, value any) Logger
	// WithError adds an error to the logger
	WithError(err error) Logger
	// GetLevel returns the current logging level
	GetLevel() Level
	// SetLevel sets the logging level
	SetLevel(level Level)
	// Sync ensures all logs are written
	Sync() error
	// GetConfig returns the current logger configuration
	GetConfig() *Config
}

// FormattedLogger defines the interface for logging formatted messages.
type FormattedLogger interface {
	// Tracef logs a message at the Trace level
	Tracef(format string, args ...any)
	// Debugf logs a message at the Debug level
	Debugf(format string, args ...any)
	// Infof logs a message at the Info level
	Infof(format string, args ...any)
	// Warnf logs a message at the Warn level
	Warnf(format string, args ...any)
	// Errorf logs a message at the Error level
	Errorf(format string, args ...any)
	// Fatalf logs a message at the Fatal level
	Fatalf(format string, args ...any)
}
