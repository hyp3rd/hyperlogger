package hyperlogger

import (
	"context"
)

// NoopLogger is a logger that does nothing.
type NoopLogger struct {
	level Level
}

// NewNoop creates a new NoopLogger.
func NewNoop() Logger {
	return &NoopLogger{
		level: InfoLevel, // Default level
	}
}

// Ensure NoopLogger implements Logger interface.
var _ Logger = (*NoopLogger)(nil)

// Basic logging methods.

// Trace logs a message at the Trace level.
func (*NoopLogger) Trace(_ string) {}

// Debug logs a message at the Debug level.
func (*NoopLogger) Debug(_ string) {}

// Info logs a message at the Info level.
func (*NoopLogger) Info(_ string) {}

// Warn logs a message at the Warn level.
func (*NoopLogger) Warn(_ string) {}

// Error logs a message at the Error level.
func (*NoopLogger) Error(_ string) {}

// Fatal logs a message at the Fatal level.
func (*NoopLogger) Fatal(_ string) {}

// Formatted logging methods.

// Tracef logs a formatted message at the Trace level.
func (*NoopLogger) Tracef(_ string, _ ...any) {}

// Debugf logs a formatted message at the Debug level.
func (*NoopLogger) Debugf(_ string, _ ...any) {}

// Infof logs a formatted message at the Info level.
func (*NoopLogger) Infof(_ string, _ ...any) {}

// Warnf logs a formatted message at the Warn level.
func (*NoopLogger) Warnf(_ string, _ ...any) {}

// Errorf logs a formatted message at the Error level.
func (*NoopLogger) Errorf(_ string, _ ...any) {}

// Fatalf logs a formatted message at the Fatal level.
func (*NoopLogger) Fatalf(_ string, _ ...any) {}

// Contextual methods - return the same logger for chaining.

// WithContext returns a new logger with the given context.
func (l *NoopLogger) WithContext(_ context.Context) Logger { return l }

// WithFields returns a new logger with the given fields.
func (l *NoopLogger) WithFields(_ ...Field) Logger { return l }

// WithField returns a new logger with the given field.
func (l *NoopLogger) WithField(_ string, _ any) Logger { return l }

// WithError returns a new logger with the given error.
func (l *NoopLogger) WithError(_ error) Logger { return l }

// Level management.

// GetLevel returns the current log level.
func (l *NoopLogger) GetLevel() Level { return l.level }

// SetLevel sets the log level.
func (l *NoopLogger) SetLevel(level Level) { l.level = level }

// Sync is a no-op operation.
func (*NoopLogger) Sync() error { return nil }

// GetConfig returns a default config.
func (l *NoopLogger) GetConfig() *Config {
	return &Config{
		Level: l.level,
	}
}
