package hyperlogger

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger/internal/utils"
)

const (
	// DefaultTimeFormat is the default time format for log entries.
	DefaultTimeFormat = time.RFC3339
	// DefaultLevel is the default logging level.
	DefaultLevel = InfoLevel
	// DefaultBufferSize is the default size of the log buffer.
	DefaultBufferSize = 4096
	// DefaultAsyncBufferSize is the default size of the async log buffer.
	DefaultAsyncBufferSize = 1024
	// LogFilePermissions are the default file permissions for log files.
	LogFilePermissions = 0o666
	// DefaultMaxFileSizeMB is the default maximum size in MB for log files before rotation.
	DefaultMaxFileSizeMB = 100
	// DefaultCompression determines if log files are compressed by default.
	DefaultCompression = true
	// DefaultSamplingInitial is the default number of logs before sampling starts.
	DefaultSamplingInitial = 100
	// DefaultSamplingThereafter is the default sampling rate (1/N) after initial logs.
	DefaultSamplingThereafter = 10
)

// SamplingConfig defines parameters for log sampling.
type SamplingConfig struct {
	// Enabled turns sampling on/off.
	Enabled bool
	// Initial is the number of entries to log before sampling starts.
	Initial int
	// Thereafter is the sampling rate (1/N) after Initial entries.
	Thereafter int
	// PerLevelThreshold when true, applies separate thresholds per level.
	PerLevelThreshold bool
}

// AsyncOverflowStrategy defines how the async writer handles a full buffer.
type AsyncOverflowStrategy uint8

const (
	// AsyncOverflowDropNewest drops the incoming log entry when the buffer is full.
	AsyncOverflowDropNewest AsyncOverflowStrategy = iota
	// AsyncOverflowBlock blocks until there is space in the buffer.
	AsyncOverflowBlock
	// AsyncOverflowDropOldest discards the oldest buffered log entry to make room for a new one.
	AsyncOverflowDropOldest
)

// IsValid reports whether the strategy value is recognised.
func (s AsyncOverflowStrategy) IsValid() bool {
	switch s {
	case AsyncOverflowDropNewest, AsyncOverflowBlock, AsyncOverflowDropOldest:
		return true
	default:
		return false
	}
}

// FileConfig holds configuration specific to file-based logging.
type FileConfig struct {
	// Path is the path to the log file
	Path string
	// MaxSizeBytes is the max size in bytes before rotation (0 = no rotation).
	MaxSizeBytes int64
	// Compress determines if rotated files should be compressed.
	Compress bool
	// MaxAge is the maximum age of log files in days before deletion (0 = no deletion).
	MaxAge int
	// MaxBackups is the maximum number of backup files to retain (0 = no limit).
	MaxBackups int
	// LocalTime uses local time instead of UTC for file names.
	LocalTime bool
	// FileMode sets the permissions for new log files.
	FileMode os.FileMode
	// CompressionLevel sets the gzip compression level (0=default, 1=best speed, 9=best compression).
	CompressionLevel int
}

// HookConfig defines a hook to be called during the logging process.
type HookConfig struct {
	// Name is the name of the hook.
	Name string
	// Levels are the log levels this hook should be triggered for.
	Levels []Level
	// Hook is the hook function to call.
	Hook Hook
}

// ContextExtractor transforms a context into structured fields.
type ContextExtractor func(ctx context.Context) []Field

// Config holds configuration for the logger.
type Config struct {
	// Level is the minimum level to log.
	Level Level
	// Output is where the logs will be written.
	Output io.Writer
	// EnableStackTrace enables stack trace for error and fatal levels.
	EnableStackTrace bool
	// EnableCaller adds the caller information to log entries.
	EnableCaller bool
	// TimeFormat specifies the format for timestamps.
	TimeFormat string
	// EnableJSON enables JSON output format.
	EnableJSON bool
	// BufferSize sets the size of the log buffer.
	BufferSize int
	// EnableAsync enables asynchronous logging for non-critical log levels.
	EnableAsync bool
	// AsyncBufferSize sets the size of the async log buffer.
	AsyncBufferSize int
	// AsyncOverflowStrategy controls how async logging behaves when the buffer is full.
	AsyncOverflowStrategy AsyncOverflowStrategy
	// AsyncDropHandler is invoked when the async writer drops a log entry.
	AsyncDropHandler func([]byte)
	// DisableTimestamp disables timestamp in log entries.
	DisableTimestamp bool
	// AdditionalFields adds these fields to all log entries.
	AdditionalFields []Field
	// Color configuration.
	Color ColorConfig
	// Encoder overrides the default encoder used for log entries.
	Encoder Encoder
	// EncoderName refers to a registered encoder to be loaded at runtime.
	EncoderName string
	// EncoderRegistry holds available encoders for name resolution.
	EncoderRegistry *EncoderRegistry
	// Sampling configures log sampling for high-volume scenarios.
	Sampling SamplingConfig
	// File configures file output settings when using file output.
	File FileConfig
	// FilePath is a convenience field for simple file output configuration.
	FilePath string
	// FileMaxSize is a convenience field for simple rotation configuration (in bytes).
	FileMaxSize int64
	// FileCompress is a convenience field for simple compression configuration.
	FileCompress bool
	// Hooks contains hooks to be called during logging.
	Hooks []HookConfig
	// ContextExtractors apply additional enrichment from context values.
	ContextExtractors []ContextExtractor
}

// DefaultConfig returns the default logger configuration.
func DefaultConfig() Config {
	return Config{
		// Set a default output destination (os.Stdout)
		Output:                os.Stdout,
		Level:                 DefaultLevel,
		EnableStackTrace:      true,
		EnableCaller:          true,
		TimeFormat:            DefaultTimeFormat,
		EnableJSON:            true,
		BufferSize:            DefaultBufferSize,
		EnableAsync:           true, // Enable async logging by default for better performance
		AsyncBufferSize:       DefaultAsyncBufferSize,
		AsyncOverflowStrategy: AsyncOverflowDropNewest,
		AdditionalFields:      make([]Field, 0), // Initialize empty slice
		Color:                 DefaultColorConfig(),
		Sampling: SamplingConfig{
			Enabled:           false,
			Initial:           DefaultSamplingInitial,
			Thereafter:        DefaultSamplingThereafter,
			PerLevelThreshold: false,
		},
		File: FileConfig{
			MaxSizeBytes:     DefaultMaxFileSizeMB * 1024 * 1024,
			Compress:         DefaultCompression,
			FileMode:         LogFilePermissions,
			CompressionLevel: -1, // Default compression level
		},
		FilePath:          "",
		FileMaxSize:       0,
		FileCompress:      false,
		EncoderRegistry:   nil,
		Hooks:             make([]HookConfig, 0),
		ContextExtractors: nil,
		Encoder:           nil,
		EncoderName:       "",
	}
}

// ProductionConfig returns a configuration optimized for production environments.
// It enables JSON output, disables colors, and sets reasonable defaults for production use.
func ProductionConfig() Config {
	config := DefaultConfig()
	config.EnableJSON = true
	config.Color.Enable = false
	config.EnableCaller = false

	return config
}

// DevelopmentConfig returns a configuration optimized for development environments.
// It enables colors, stack traces, and sets a lower log level for more verbose output.
func DevelopmentConfig() Config {
	config := DefaultConfig()
	config.EnableJSON = false
	config.Color.Enable = true
	config.Level = DebugLevel
	config.EnableStackTrace = true
	config.EnableCaller = true

	return config
}

// SetOutput sets the output destination for the logger. It accepts "stdout", "stderr",
// or a file path as input. If a file path is provided, it will create the file if it
// doesn't exist and open it in append mode. The function returns the io.Writer for
// the selected output and any error that occurred.
func SetOutput(output string) (io.Writer, error) {
	// Normalize the input to lowercase for case-insensitive comparison
	switch strings.ToLower(output) {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		path := filepath.Clean(output)

		if path == "" {
			return nil, ewrap.New("output path cannot be empty")
		}

		if !filepath.IsAbs(path) {
			securePath, err := utils.SecurePath(path)
			if err != nil {
				return nil, ewrap.Wrap(err, "invalid output path")
			}

			path = securePath
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, LogFilePermissions)
		if err != nil {
			return nil, ewrap.Wrapf(err, "failed to open log file %s", path)
		}

		return file, nil
	}
}

// ParseLevel parses the given log level string and returns the normalized
// lowercase string representation of the level, or an error if the level is
// invalid.
func ParseLevel(level string) (string, error) {
	// Normalize the input to lowercase for case-insensitive comparison
	switch strings.ToLower(level) {
	case "trace":
		return "trace", nil
	case "debug":
		return "debug", nil
	case "info":
		return "info", nil
	case "warn", "warning":
		return "warn", nil
	case "error":
		return "error", nil
	case "panic":
		return "panic", nil
	case "fatal":
		return "fatal", nil
	default:
		return "", ewrap.New("invalid log level: " + level)
	}
}
