package hyperlogger

import (
	"io"
	"os"
	"time"
)

// ConfigBuilder provides a fluent API for constructing logger configurations.
// It allows for more readable and chainable configuration setup.
type ConfigBuilder struct {
	config Config
}

// NewConfigBuilder creates a new builder with sensible defaults.
// This is the entry point for the fluent configuration API.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: Config{
			Output:                os.Stdout,
			Level:                 InfoLevel,
			TimeFormat:            time.RFC3339,
			EnableCaller:          true,
			AsyncBufferSize:       DefaultAsyncBufferSize,
			AsyncOverflowStrategy: AsyncOverflowDropNewest,
			Color: ColorConfig{
				Enable:   true,
				ForceTTY: false,
				LevelColors: map[Level]string{
					TraceLevel: Cyan,
					DebugLevel: Blue,
					InfoLevel:  Green,
					WarnLevel:  Yellow,
					ErrorLevel: Red,
					FatalLevel: Magenta,
				},
			},
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
			ContextExtractors: make([]ContextExtractor, 0),
		},
	}
}

// WithOutput sets the output destination.
// Example: builder.WithOutput(os.Stderr).
func (b *ConfigBuilder) WithOutput(output io.Writer) *ConfigBuilder {
	b.config.Output = output

	return b
}

// WithConsoleOutput configures the logger to write to the console (stdout).
// This is a convenience method for WithOutput(os.Stdout).
func (b *ConfigBuilder) WithConsoleOutput() *ConfigBuilder {
	b.config.Output = os.Stdout

	return b
}

// WithFileOutput configures the logger to write to a file.
// The file will be created if it doesn't exist, and appended to if it does.
// Example: builder.WithFileOutput("/var/log/my_app.log").
func (b *ConfigBuilder) WithFileOutput(path string) *ConfigBuilder {
	// The actual file writer will be created when Build() is called
	// For now, just store the path
	b.config.File.Path = path

	return b
}

// WithLevel sets the logging level.
// Example: builder.WithLevel(logger.DebugLevel).
func (b *ConfigBuilder) WithLevel(level Level) *ConfigBuilder {
	b.config.Level = level

	return b
}

// WithDebugLevel is a convenience method for WithLevel(DebugLevel).
func (b *ConfigBuilder) WithDebugLevel() *ConfigBuilder {
	return b.WithLevel(DebugLevel)
}

// WithInfoLevel is a convenience method for WithLevel(InfoLevel).
func (b *ConfigBuilder) WithInfoLevel() *ConfigBuilder {
	return b.WithLevel(InfoLevel)
}

// WithTimeFormat sets the time format string.
// Example: builder.WithTimeFormat(time.RFC3339).
func (b *ConfigBuilder) WithTimeFormat(format string) *ConfigBuilder {
	b.config.TimeFormat = format

	return b
}

// WithNoTimestamp disables timestamp output in log entries.
func (b *ConfigBuilder) WithNoTimestamp() *ConfigBuilder {
	b.config.DisableTimestamp = true

	return b
}

// WithCaller enables or disables including the caller information.
// Example: builder.WithCaller(true).
func (b *ConfigBuilder) WithCaller(enable bool) *ConfigBuilder {
	b.config.EnableCaller = enable

	return b
}

// WithStackTrace enables or disables including stack traces for errors.
// Example: builder.WithStackTrace(true).
func (b *ConfigBuilder) WithStackTrace(enable bool) *ConfigBuilder {
	b.config.EnableStackTrace = enable

	return b
}

// WithJSONFormat enables JSON formatting for log entries.
// Example: builder.WithJSONFormat(true).
func (b *ConfigBuilder) WithJSONFormat(enable bool) *ConfigBuilder {
	b.config.EnableJSON = enable

	return b
}

// WithColors enables or disables color output.
// Example: builder.WithColors(true).
func (b *ConfigBuilder) WithColors(enable bool) *ConfigBuilder {
	b.config.Color.Enable = enable

	return b
}

// WithForceColors forces color output even when not writing to a terminal.
// Example: builder.WithForceColors(true).
func (b *ConfigBuilder) WithForceColors(force bool) *ConfigBuilder {
	b.config.Color.ForceTTY = force

	return b
}

// WithAsyncBufferSize sets the buffer size for asynchronous logging.
// Example: builder.WithAsyncBufferSize(1000).
func (b *ConfigBuilder) WithAsyncBufferSize(size int) *ConfigBuilder {
	b.config.AsyncBufferSize = size

	return b
}

// WithAsyncOverflowStrategy sets the behaviour when the async buffer is full.
func (b *ConfigBuilder) WithAsyncOverflowStrategy(strategy AsyncOverflowStrategy) *ConfigBuilder {
	b.config.AsyncOverflowStrategy = strategy

	return b
}

// WithAsyncDropHandler sets the handler invoked when async logging drops an entry.
func (b *ConfigBuilder) WithAsyncDropHandler(handler func([]byte)) *ConfigBuilder {
	b.config.AsyncDropHandler = handler

	return b
}

// WithEncoder assigns a custom encoder to the configuration.
func (b *ConfigBuilder) WithEncoder(encoder Encoder) *ConfigBuilder {
	b.config.Encoder = encoder

	return b
}

// WithEncoderName selects an encoder by name from the global registry.
func (b *ConfigBuilder) WithEncoderName(name string) *ConfigBuilder {
	b.config.EncoderName = name

	return b
}

// WithEncoderRegistry sets the encoder registry used when resolving named encoders.
func (b *ConfigBuilder) WithEncoderRegistry(registry *EncoderRegistry) *ConfigBuilder {
	b.config.EncoderRegistry = registry

	return b
}

// WithContextExtractor appends a context extractor used to enrich log fields.
func (b *ConfigBuilder) WithContextExtractor(extractor ContextExtractor) *ConfigBuilder {
	if extractor != nil {
		b.config.ContextExtractors = append(b.config.ContextExtractors, extractor)
	}

	return b
}

// WithContextExtractors appends multiple context extractors.
func (b *ConfigBuilder) WithContextExtractors(extractors ...ContextExtractor) *ConfigBuilder {
	for _, extractor := range extractors {
		b.WithContextExtractor(extractor)
	}

	return b
}

// WithField adds a field to the log entries.
// Example: builder.WithField("version", "1.0.0").
func (b *ConfigBuilder) WithField(key string, value any) *ConfigBuilder {
	b.config.AdditionalFields = append(b.config.AdditionalFields, Field{
		Key:   key,
		Value: value,
	})

	return b
}

// WithFields adds multiple fields to the log entries.
// Example: builder.WithFields([]Field{{Key: "version", Value: "1.0.0"}}).
func (b *ConfigBuilder) WithFields(fields []Field) *ConfigBuilder {
	b.config.AdditionalFields = append(b.config.AdditionalFields, fields...)

	return b
}

// WithFileRotation configures log file rotation.
// Example: builder.WithFileRotation(100*1024*1024, true).
func (b *ConfigBuilder) WithFileRotation(maxSizeBytes int64, compress bool) *ConfigBuilder {
	b.config.File.MaxSizeBytes = maxSizeBytes
	b.config.File.Compress = compress

	return b
}

// WithSampling configures log sampling for high-volume logging.
// Initial is the number of logs to allow before sampling starts.
// Thereafter is the sampling rate (1/N) after the initial threshold.
// PerLevel applies separate thresholds for each log level.
// Example: builder.WithSampling(true, 1000, 100, false).
func (b *ConfigBuilder) WithSampling(enabled bool, initial, thereafter int, perLevel bool) *ConfigBuilder {
	b.config.Sampling.Enabled = enabled
	b.config.Sampling.Initial = initial
	b.config.Sampling.Thereafter = thereafter
	b.config.Sampling.PerLevelThreshold = perLevel

	return b
}

// WithHook adds a hook to be called during logging.
// Example: builder.WithHook("metrics", NewMetricsHook([]Level{ErrorLevel, FatalLevel})).
func (b *ConfigBuilder) WithHook(name string, hook Hook) *ConfigBuilder {
	b.config.Hooks = append(b.config.Hooks, HookConfig{
		Name:   name,
		Hook:   hook,
		Levels: hook.Levels(),
	})

	return b
}

// WithFileCompression configures compression level for rotated files
// Level values:
// -1 = Default compression (good compromise between speed and compression)
// 0 = No compression
// 1 = Best speed
// 9 = Best compression
// Example: builder.WithFileCompression(1) // Fast compression.
func (b *ConfigBuilder) WithFileCompression(level int) *ConfigBuilder {
	b.config.File.CompressionLevel = level

	return b
}

// WithFileRetention configures retention policies for log files
// Example: builder.WithFileRetention(7, 10) // Keep logs for 7 days, max 10 files.
func (b *ConfigBuilder) WithFileRetention(maxAgeDays, maxFiles int) *ConfigBuilder {
	b.config.File.MaxAge = maxAgeDays
	b.config.File.MaxBackups = maxFiles

	return b
}

// WithEnableAsync enables or disables asynchronous logging.
// Example: builder.WithEnableAsync(true).
// This is useful for high-throughput applications.
// It uses a goroutine to write logs in the background, improving performance.
// However, it may introduce some latency in log delivery.
// The buffer size can be configured using WithAsyncBufferSize.
func (b *ConfigBuilder) WithEnableAsync(enabled bool) *ConfigBuilder {
	b.config.EnableAsync = enabled

	return b
}

// WithLocalDefaults configures the logger with sensible defaults for local development.
// This enables debug level, caller info, colorized output, and text (non-JSON) format.
func (b *ConfigBuilder) WithLocalDefaults() *ConfigBuilder {
	return b.
		WithDebugLevel().
		WithCaller(true).
		WithColors(true).
		WithJSONFormat(false).
		WithStackTrace(true)
}

// WithDevelopmentDefaults configures the logger with sensible defaults for development.
// This enables debug level, caller info, no colors, and JSON format.
func (b *ConfigBuilder) WithDevelopmentDefaults() *ConfigBuilder {
	return b.
		WithDebugLevel().
		WithCaller(true).
		WithColors(false).
		WithJSONFormat(true).
		WithStackTrace(true)
}

// WithProductionDefaults configures the logger with sensible defaults for production.
// This enables info level, no caller info, no colors, and JSON format.
func (b *ConfigBuilder) WithProductionDefaults() *ConfigBuilder {
	return b.
		WithInfoLevel().
		WithCaller(false).
		WithColors(false).
		WithJSONFormat(true)
}

// Build creates a Config object from the builder.
func (b *ConfigBuilder) Build() *Config {
	config := b.config

	return &config
}
