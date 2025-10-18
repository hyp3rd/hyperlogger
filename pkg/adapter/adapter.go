// Package adapter provides concrete implementations of the logger interface.
//
// The adapter package bridges the abstract Logger interface with concrete implementations
// that format and output log messages. It handles buffering, formatting, and writing
// log entries to various output destinations.
package adapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyp3rd/ewrap"
	"github.com/mattn/go-isatty"

	"github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/internal/constants"
	"github.com/hyp3rd/hyperlogger/internal/output"
)

// Buffer pool sizes for different message sizes.
const (
	smallBufferSize  = 1024
	mediumBufferSize = 4096
	largeBufferSize  = 16384
	xlargeBufferSize = 32768

	// Predicted sizes by message type to reduce reallocations.
	jsonBaseSize     = 200 // Base size for JSON messages
	consoleBaseSize  = 100 // Base size for console messages
	fieldOverhead    = 40  // Estimated overhead per field in JSON
	consoleFieldSize = 25  // Estimated size per field in console output

	// Reuse threshold - only reuse buffers if they're within this ratio of the expected size.
	bufferReuseRatio = 0.6

	callerSkipLevel   = 5 // Skip level for caller info (increased to properly show external callers)
	charactersPadding = 5

	// Repeated values.
	unknown = "unknown"
)

const (
	// ASCII control characters and printable range.
	asciiControlStart = 32  // Start of ASCII printable characters (space)
	asciiControlEnd   = 126 // End of ASCII printable characters (~)
)

const fieldSnapshotSlack = 4 // Extra capacity to reduce frequent reallocations

var (
	// ErrNoFilePathSet indicates that the file path is not set for file output.
	ErrNoFilePathSet = ewrap.New("file path not set for file output")
	// ErrEncoderNotFound indicates that a requested encoder could not be resolved.
	ErrEncoderNotFound = ewrap.New("encoder not found")
)

//nolint:gochecknoglobals
var contextKeyPairs = []struct {
	field string
	key   any
}{
	{field: "namespace", key: constants.NamespaceKey{}},
	{field: "service", key: constants.ServiceKey{}},
	{field: "environment", key: constants.EnvironmentKey{}},
	{field: "application", key: constants.ApplicationKey{}},
	{field: "component", key: constants.ComponentKey{}},
	{field: "request", key: constants.RequestKey{}},
	{field: "user", key: constants.UserKey{}},
	{field: "session_id", key: constants.SessionKey{}},
	{field: "trace_id", key: constants.TraceKey{}},
}

// bufferPoolBucket represents a size category for buffer pooling.
type bufferPoolBucket struct {
	size int
	pool sync.Pool
}

// newBufferPool creates the pooled buffers used to minimize allocations per log entry.
func newBufferPool() []*bufferPoolBucket {
	return []*bufferPoolBucket{
		{
			size: smallBufferSize,
			pool: sync.Pool{
				New: func() any {
					return bytes.NewBuffer(make([]byte, 0, smallBufferSize))
				},
			},
		},
		{
			size: mediumBufferSize,
			pool: sync.Pool{
				New: func() any {
					return bytes.NewBuffer(make([]byte, 0, mediumBufferSize))
				},
			},
		},
		{
			size: largeBufferSize,
			pool: sync.Pool{
				New: func() any {
					return bytes.NewBuffer(make([]byte, 0, largeBufferSize))
				},
			},
		},
		{
			size: xlargeBufferSize,
			pool: sync.Pool{
				New: func() any {
					return bytes.NewBuffer(make([]byte, 0, xlargeBufferSize))
				},
			},
		},
	}
}

// Adapter implements the hyperlogger.Logger interface.
//
//nolint:containedctx
type Adapter struct {
	config       *hyperlogger.Config
	hookRegistry *hyperlogger.HookRegistry
	level        *atomic.Uint32
	ctx          context.Context
	fields       []hyperlogger.Field
	sampler      *logSampler
	encoder      hyperlogger.Encoder

	// Unified buffer pool system
	bufferPool []*bufferPoolBucket

	fieldsPool sync.Pool
}

// NewAdapter creates a new logger adapter with the given configuration.
func NewAdapter(ctx context.Context, config hyperlogger.Config) (hyperlogger.Logger, error) {
	err := prepareOutput(&config)
	if err != nil {
		return nil, err
	}

	err = validateConfig(&config)
	if err != nil {
		return nil, err
	}

	enc, err := resolveEncoder(&config)
	if err != nil {
		return nil, err
	}

	normalizedCtx := normalizeContext(ctx)

	// Initialize adapter
	adapter := &Adapter{
		config:       &config,
		level:        new(atomic.Uint32),
		hookRegistry: hyperlogger.NewHookRegistry(),
		fields:       cloneFields(config.AdditionalFields),
		ctx:          normalizedCtx,
		encoder:      enc,
	}

	adapter.fieldsPool = sync.Pool{
		New: func() any {
			slice := make([]hyperlogger.Field, 0, len(adapter.fields)+fieldSnapshotSlack)

			return &slice
		},
	}

	adapter.level.Store(uint32(config.Level))

	// Set up unified buffer pool with multiple size buckets
	adapter.bufferPool = newBufferPool()

	adapter.sampler = newLogSampler(config.Sampling)

	// Wrap output in AsyncWriter if async logging is enabled
	if config.EnableAsync {
		asyncConfig := output.AsyncConfig{
			BufferSize:       config.AsyncBufferSize,
			WaitTimeout:      constants.DefaultTimeout,
			ErrorHandler:     func(err error) { fmt.Fprintf(os.Stderr, "Error in async logger: %v\n", err) },
			OverflowStrategy: convertOverflowStrategy(config.AsyncOverflowStrategy),
			DropHandler:      config.AsyncDropHandler,
			MetricsReporter: func(metrics output.AsyncMetrics) {
				mapped := toAsyncMetrics(metrics)
				if config.AsyncMetricsHandler != nil {
					config.AsyncMetricsHandler(normalizedCtx, mapped)
				}

				hyperlogger.EmitAsyncMetrics(normalizedCtx, mapped)
			},
		}

		adapter.config.Output = output.NewAsyncWriter(config.Output, asyncConfig)
	}

	// Register hooks from config
	for _, hookConfig := range config.Hooks {
		err := adapter.hookRegistry.AddHook(hookConfig.Name, hookConfig.Hook)
		if err != nil {
			return nil, ewrap.Wrapf(err, "failed to register hook '%s'", hookConfig.Name)
		}
	}

	return adapter, nil
}

// Sync ensures that all logs have been written.
// Flushes the AsyncWriter if one is being used.
func (a *Adapter) Sync() error {
	// If using AsyncWriter, we need to call its Sync method
	if a.config.EnableAsync {
		if asyncWriter, ok := a.config.Output.(*output.AsyncWriter); ok {
			return asyncWriter.Sync()
		}
	}

	// For synchronous writers or if type assertion failed.
	if syncer, ok := a.config.Output.(interface{ Sync() error }); ok {
		// Check if we're trying to sync stdout/stderr and skip it
		if f, ok := a.config.Output.(*os.File); ok {
			if f == os.Stdout || f == os.Stderr {
				return nil // Skip syncing stdout/stderr
			}
		}

		return syncer.Sync()
	}

	return nil
}

// Trace logs a message at trace level.
func (a *Adapter) Trace(msg string) {
	a.log(hyperlogger.TraceLevel, msg)
}

// Debug logs a message at debug level.
func (a *Adapter) Debug(msg string) {
	a.log(hyperlogger.DebugLevel, msg)
}

// Info logs a message at info level.
func (a *Adapter) Info(msg string) {
	a.log(hyperlogger.InfoLevel, msg)
}

// Warn logs a message at warn level.
func (a *Adapter) Warn(msg string) {
	a.log(hyperlogger.WarnLevel, msg)
}

// Error logs a message at error level.
func (a *Adapter) Error(msg string) {
	a.log(hyperlogger.ErrorLevel, msg)
}

// Fatal logs a message at fatal level then calls os.Exit(1).
func (a *Adapter) Fatal(msg string) {
	a.log(hyperlogger.FatalLevel, msg)
}

// Tracef logs a formatted message at trace level.
func (a *Adapter) Tracef(format string, args ...any) {
	a.Trace(fmt.Sprintf(format, args...))
}

// Debugf logs a formatted message at debug level.
func (a *Adapter) Debugf(format string, args ...any) {
	a.Debug(fmt.Sprintf(format, args...))
}

// Infof logs a formatted message at info level.
func (a *Adapter) Infof(format string, args ...any) {
	a.Info(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted message at warn level.
func (a *Adapter) Warnf(format string, args ...any) {
	a.Warn(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted message at error level.
func (a *Adapter) Errorf(format string, args ...any) {
	a.Error(fmt.Sprintf(format, args...))
}

// Fatalf logs a formatted message at fatal level.
func (a *Adapter) Fatalf(format string, args ...any) {
	a.Fatal(fmt.Sprintf(format, args...))
}

// WithContext returns a new logger with the given context.
func (a *Adapter) WithContext(ctx context.Context) hyperlogger.Logger {
	ctx = normalizeContext(ctx)

	// Create a new adapter with the same config but with the given context
	newAdapter := &Adapter{
		config:       a.config,
		level:        a.level,
		fields:       a.fields,
		hookRegistry: a.hookRegistry,
		ctx:          ctx,
		bufferPool:   a.bufferPool,
		sampler:      a.sampler,
		encoder:      a.encoder,
	}

	return newAdapter
}

// WithField adds a field to the hyperlogger.
func (a *Adapter) WithField(key string, value any) hyperlogger.Logger {
	return a.WithFields(hyperlogger.Field{Key: key, Value: value})
}

// WithFields adds fields to the hyperlogger.
func (a *Adapter) WithFields(fields ...hyperlogger.Field) hyperlogger.Logger {
	if len(fields) == 0 {
		return a
	}

	// Create a new adapter with the same config but merged fields
	newAdapter := &Adapter{
		config:       a.config,
		level:        a.level,
		fields:       mergeFields(a.fields, fields),
		hookRegistry: a.hookRegistry,
		ctx:          a.ctx,
		bufferPool:   a.bufferPool,
		sampler:      a.sampler,
		encoder:      a.encoder,
	}

	return newAdapter
}

// WithError adds an error field to the hyperlogger.
func (a *Adapter) WithError(err error) hyperlogger.Logger {
	if err == nil {
		return a
	}

	return a.WithField("error", err.Error())
}

// GetLevel returns the current logging level.
func (a *Adapter) GetLevel() hyperlogger.Level {
	//nolint:gosec // The log levels can't be changed at runtime and cause integer overflow conversion.
	return hyperlogger.Level(a.level.Load())
}

// SetLevel sets the logging level.
func (a *Adapter) SetLevel(level hyperlogger.Level) {
	if level.IsValid() {
		a.level.Store(uint32(level))
	}
}

// GetConfig returns the current logger configuration.
func (a *Adapter) GetConfig() *hyperlogger.Config {
	return a.config
}

// validateConfig validates the logger configuration and sets defaults for missing values.
func validateConfig(config *hyperlogger.Config) error {
	if config == nil {
		return ewrap.New("logger config cannot be nil")
	}

	// Check if the level is valid
	if !config.Level.IsValid() {
		return ewrap.New("invalid log level").WithMetadata("level", config.Level)
	}

	// Set default time format if not specified
	if config.TimeFormat == "" {
		config.TimeFormat = time.RFC3339
	}

	// Check if output is specified
	if config.Output == nil {
		return ewrap.New("output writer is required")
	}

	enforceColorPolicy(config)

	if !config.AsyncOverflowStrategy.IsValid() {
		config.AsyncOverflowStrategy = hyperlogger.AsyncOverflowDropNewest
	}

	return nil
}

func enforceColorPolicy(config *hyperlogger.Config) {
	if config == nil {
		return
	}

	if !config.Color.Enable || config.Color.ForceTTY {
		return
	}

	if hasTTY(config.Output) {
		return
	}

	config.Color.Enable = false
}

func hasTTY(writer io.Writer) bool {
	if writer == nil {
		return false
	}

	switch typedWriter := writer.(type) {
	case *output.MultiWriter:
		for _, inner := range typedWriter.Writers {
			if hasTTY(inner) {
				return true
			}
		}

		return false
	case interface{ Underlying() io.Writer }:
		return hasTTY(typedWriter.Underlying())
	case *output.ConsoleWriter:
		return true
	case interface{ Fd() uintptr }:
		fd := typedWriter.Fd()

		return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
	default:
		return false
	}
}

// cloneFields creates a shallow copy of a slice of fields to prevent shared state mutation.
func cloneFields(fields []hyperlogger.Field) []hyperlogger.Field {
	if len(fields) == 0 {
		return nil
	}

	cloned := make([]hyperlogger.Field, len(fields))
	copy(cloned, fields)

	return cloned
}

func (a *Adapter) borrowFieldsSnapshot() ([]hyperlogger.Field, *[]hyperlogger.Field) {
	baseLen := len(a.fields)
	raw := a.fieldsPool.Get()

	var (
		storage *[]hyperlogger.Field
		ok      bool
	)

	if raw == nil {
		slice := make([]hyperlogger.Field, 0, baseLen)
		storage = &slice
	} else {
		storage, ok = raw.(*[]hyperlogger.Field)
		if !ok {
			storage = &[]hyperlogger.Field{}
		}
	}

	fields := *storage
	if cap(fields) < baseLen {
		fields = make([]hyperlogger.Field, baseLen)
	} else {
		fields = fields[:baseLen]
	}

	copy(fields, a.fields)
	*storage = fields

	return fields, storage
}

func (a *Adapter) releaseFieldsSnapshot(storage *[]hyperlogger.Field) {
	fields := *storage
	for i := range fields {
		fields[i] = hyperlogger.Field{}
	}

	fields = fields[:0]
	*storage = fields
	a.fieldsPool.Put(storage)
}

func prepareOutput(config *hyperlogger.Config) error {
	fileWriter, err := buildFileWriter(config)
	if err != nil && !errors.Is(err, ErrNoFilePathSet) {
		return err
	}
	//nolint:revive // enforce-switch-style: switch must have a default case clause: no need.
	switch {
	case config.Output == nil && fileWriter != nil:
		config.Output = fileWriter
	case config.Output != nil && fileWriter != nil:
		baseWriter, err := toOutputWriter(config.Output)
		if err != nil {
			return err
		}

		multi, err := output.NewMultiWriter(baseWriter, fileWriter)
		if err != nil {
			return err
		}

		config.Output = multi
	}

	return nil
}

func resolveEncoder(config *hyperlogger.Config) (hyperlogger.Encoder, error) {
	if config != nil && config.Encoder != nil {
		return config.Encoder, nil
	}

	registry, err := ensureEncoderRegistry(config)
	if err != nil {
		return nil, err
	}

	if name := encoderName(config); name != "" {
		if enc, ok := registry.Get(name); ok {
			return enc, nil
		}

		return nil, ewrap.Wrap(ErrEncoderNotFound, "encoder not registered").WithMetadata("name", name)
	}

	for _, candidate := range encoderCandidates(config) {
		if enc, ok := registry.Get(candidate); ok {
			return enc, nil
		}
	}

	return nil, ErrEncoderNotFound
}

func ensureEncoderRegistry(config *hyperlogger.Config) (*hyperlogger.EncoderRegistry, error) {
	if config == nil {
		return newEncoderRegistryWithDefaults()
	}

	if config.EncoderRegistry == nil {
		registry, err := newEncoderRegistryWithDefaults()
		if err != nil {
			return nil, err
		}

		config.EncoderRegistry = registry

		return registry, nil
	}

	err := registerDefaultEncoders(config.EncoderRegistry)
	if err != nil {
		return nil, err
	}

	return config.EncoderRegistry, nil
}

func encoderName(config *hyperlogger.Config) string {
	if config == nil {
		return ""
	}

	return config.EncoderName
}

func encoderCandidates(config *hyperlogger.Config) []string {
	if config == nil {
		return []string{jsonEncoderName, consoleEncoderName}
	}

	if config.EnableJSON {
		return []string{jsonEncoderName, consoleEncoderName}
	}

	return []string{consoleEncoderName, jsonEncoderName}
}

func buildFileWriter(config *hyperlogger.Config) (output.Writer, error) {
	path := config.File.Path

	if path == "" {
		return nil, ErrNoFilePathSet
	}

	fileCfg := output.FileConfig{
		Path:     path,
		Compress: config.File.Compress,
	}

	if maxSize := config.File.MaxSizeBytes; maxSize > 0 {
		fileCfg.MaxSize = maxSize
	}

	if mode := config.File.FileMode; mode != 0 {
		fileCfg.FileMode = mode
	}

	writer, err := output.NewFileWriter(fileCfg)
	if err != nil {
		return nil, err
	}

	config.File.Path = path
	config.File.MaxSizeBytes = fileCfg.MaxSize
	config.File.Compress = fileCfg.Compress
	config.FilePath = path
	config.FileMaxSize = fileCfg.MaxSize
	config.FileCompress = fileCfg.Compress

	return writer, nil
}

func toOutputWriter(w io.Writer) (output.Writer, error) {
	if w == nil {
		return nil, ewrap.New("output writer cannot be nil")
	}

	if ow, ok := w.(output.Writer); ok {
		return ow, nil
	}

	return output.NewWriterAdapter(w), nil
}

func convertOverflowStrategy(strategy hyperlogger.AsyncOverflowStrategy) output.AsyncOverflowStrategy {
	//nolint:exhaustive // output.AsyncOverflowDropNewest is the default behavior
	switch strategy {
	case hyperlogger.AsyncOverflowBlock:
		return output.AsyncOverflowBlock
	case hyperlogger.AsyncOverflowDropOldest:
		return output.AsyncOverflowDropOldest
	case hyperlogger.AsyncOverflowHandoff:
		return output.AsyncOverflowHandoff
	default:
		return output.AsyncOverflowDropNewest
	}
}

func toAsyncMetrics(metrics output.AsyncMetrics) hyperlogger.AsyncMetrics {
	return hyperlogger.AsyncMetrics{
		Enqueued:   metrics.Enqueued,
		Processed:  metrics.Processed,
		Dropped:    metrics.Dropped,
		WriteError: metrics.WriteError,
		Retried:    metrics.Retried,
		QueueDepth: metrics.QueueDepth,
		Bypassed:   metrics.Bypassed,
	}
}

// mergeFields combines multiple field slices into a single slice, avoiding duplicates.
// Fields from later slices override those from earlier ones if they have the same key.
func mergeFields(fields ...[]hyperlogger.Field) []hyperlogger.Field {
	if len(fields) == 0 {
		return nil
	}

	// Count total capacity needed
	totalCap := 0
	for _, fs := range fields {
		totalCap += len(fs)
	}

	// Create map to track field keys for de-duplication
	seen := make(map[string]int, totalCap)
	result := make([]hyperlogger.Field, 0, totalCap)

	// Merge all field slices
	for _, fs := range fields {
		for _, field := range fs {
			if idx, exists := seen[field.Key]; exists {
				// Update existing field
				result[idx].Value = field.Value
			} else {
				// Add new field
				seen[field.Key] = len(result)
				result = append(result, field)
			}
		}
	}

	return result
}

// getLevelColor returns the color code for a log level and a boolean
// indicating whether color should be applied.
func getLevelColor(level hyperlogger.Level) (output.ColorCode, bool) {
	switch level {
	case hyperlogger.TraceLevel:
		return output.ColorCodeMagenta, true
	case hyperlogger.DebugLevel:
		return output.ColorCodeBlue, true
	case hyperlogger.InfoLevel:
		return output.ColorCodeGreen, true
	case hyperlogger.WarnLevel:
		return output.ColorCodeYellow, true
	case hyperlogger.ErrorLevel:
		return output.ColorCodeRed, true
	case hyperlogger.FatalLevel:
		return output.ColorCodeRedBold, true
	default:
		return output.ColorCodeReset, false
	}
}

// getBuffer retrieves an appropriately-sized buffer from the pool.
// Uses predictBufferSize to get an optimal buffer for the content.
func (a *Adapter) getBuffer(size int) *bytes.Buffer {
	// Find the smallest bucket that can accommodate the requested size
	for _, bucket := range a.bufferPool {
		if size <= bucket.size {
			if buf, ok := bucket.pool.Get().(*bytes.Buffer); ok {
				buf.Reset()

				return buf
			}
			// If type assertion fails, create a new buffer of this size
			return bytes.NewBuffer(make([]byte, 0, bucket.size))
		}
	}

	// If size exceeds all buckets, create a new buffer with exact capacity
	return bytes.NewBuffer(make([]byte, 0, size))
}

// returnBuffer returns a buffer to the appropriate pool.
func (a *Adapter) returnBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	bufCap := buf.Cap()

	// Find the appropriate bucket for this buffer size
	// If buffer is larger than any bucket, don't return it to the pool
	// Let it be garbage collected
	for _, bucket := range a.bufferPool {
		if bufCap <= bucket.size {
			// Only return the buffer if it's not too small for the bucket
			// This prevents excessive buffer growth
			if float64(bufCap) >= float64(bucket.size)*bufferReuseRatio {
				bucket.pool.Put(buf)
			}

			return
		}
	}
}

// getCaller returns the caller information at the specified skip level.
func getCaller(skip int) (string, int, string) {
	// Get caller information
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return unknown, 0, unknown
	}

	// Get function name
	fn := runtime.FuncForPC(pc)

	var funcName string

	if fn == nil {
		funcName = unknown
	} else {
		funcName = filepath.Base(fn.Name())
	}

	return file, line, funcName
}

// formatCallerInfo formats the caller information based on configuration.
func formatCallerInfo(file string, line int, funcName string, shortPath bool) string {
	if shortPath {
		file = filepath.Base(file)
	}

	return fmt.Sprintf("%s:%d %s", file, line, funcName)
}

func normalizeContext(ctx context.Context) context.Context {
	switch ctx {
	case nil, context.Background(), context.TODO():
		return context.Background()
	default:
		return ctx
	}
}

// withContext adds context values to the fields.
func withContext(ctx context.Context, fields []hyperlogger.Field, cfg *hyperlogger.Config) []hyperlogger.Field {
	if ctx == nil {
		return fields
	}

	var extras []hyperlogger.Field

	extras = extractContextKeys(ctx, extras)

	if global := hyperlogger.GlobalContextExtractors(); len(global) > 0 {
		extras = append(extras, hyperlogger.ApplyContextExtractors(ctx, global...)...)
	}

	if cfg != nil {
		extras = append(extras, hyperlogger.ApplyContextExtractors(ctx, cfg.ContextExtractors...)...)
	}

	if len(extras) == 0 {
		return fields
	}

	return mergeFields(fields, extras)
}

// extractContextKeys extracts known context keys and adds them as fields.
func extractContextKeys(ctx context.Context, extras []hyperlogger.Field) []hyperlogger.Field {
	if ctx == nil {
		return extras
	}

	for _, pair := range contextKeyPairs {
		if v, ok := ctx.Value(pair.key).(string); ok && v != "" {
			extras = append(extras, hyperlogger.Field{Key: pair.field, Value: v})
		}
	}

	return extras
}

// attachStackTrace appends a stack trace to the entry when enabled for high-severity logs.
func (a *Adapter) attachStackTrace(entry *hyperlogger.Entry) {
	if entry == nil || a == nil || a.config == nil {
		return
	}

	if !a.config.EnableStackTrace {
		return
	}

	if entry.Level < hyperlogger.ErrorLevel {
		return
	}

	stack := debug.Stack()
	if len(stack) == 0 {
		return
	}

	entry.Fields = append(entry.Fields, hyperlogger.Field{
		Key:   "stack",
		Value: string(stack),
	})
}

// formatValue formats a value for logging.
func formatValue(v any) string {
	if v == nil {
		return "null"
	}

	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, complex64, complex128:
		return fmt.Sprintf("%v", val)
	case bool:
		return strconv.FormatBool(val)
	case time.Time:
		return val.Format(time.RFC3339)
	case fmt.Stringer:
		return val.String()
	case error:
		return val.Error()
	default:
		// For more complex types, use %+v
		return fmt.Sprintf("%+v", val)
	}
}

// formatConsoleOutput formats log entry for console output.
// Optimized to minimize allocations and string operations.
func formatConsoleOutput(
	builder *bytes.Buffer,
	entry *hyperlogger.Entry,
	config *hyperlogger.Config,
) {
	cfg := config
	if cfg == nil {
		cfg = &hyperlogger.Config{}
	}

	// Format timestamp when enabled
	appendTimestamp(builder, cfg.TimeFormat, cfg.DisableTimestamp)

	// Format log level with configured colors
	appendLogLevel(builder, entry.Level, cfg.Color)

	// Add caller information if enabled
	if cfg.EnableCaller {
		file, line, funcName := getCaller(callerSkipLevel)
		appendCallerInfo(builder, file, line, funcName)
	}

	// Add message
	builder.WriteString(entry.Message)

	// Add fields if present
	if len(entry.Fields) > 0 {
		appendFields(builder, entry.Fields)
	}

	builder.WriteByte('\n')
}

// appendTimestamp writes the formatted timestamp to the buffer.
func appendTimestamp(builder *bytes.Buffer, timeFormat string, disable bool) {
	if builder == nil || disable {
		return
	}

	if timeFormat == "" {
		timeFormat = time.RFC3339
	}

	builder.WriteString(time.Now().Format(timeFormat))
	builder.WriteByte(' ')
}

// appendLogLevel formats and writes the log level to the buffer.
func appendLogLevel(builder *bytes.Buffer, level hyperlogger.Level, colorCfg hyperlogger.ColorConfig) {
	levelStr := level.String()

	if colorCfg.Enable {
		if seq, ok := colorCfg.LevelColors[level]; ok && seq != "" {
			builder.WriteString(seq)
			appendPaddedLevel(builder, levelStr)
			builder.WriteString(hyperlogger.Reset)

			return
		}

		if color, ok := getLevelColor(level); ok {
			builder.WriteString(color.Start())
			appendPaddedLevel(builder, levelStr)
			builder.WriteString(color.End())

			return
		}
	}

	appendPaddedLevel(builder, levelStr)
}

// appendPaddedLevel writes the level string padded to 5 characters.
func appendPaddedLevel(builder *bytes.Buffer, levelStr string) {
	builder.WriteByte('[')

	// Pad level string to 5 characters for alignment
	padLen := charactersPadding - len(levelStr)

	for range padLen {
		builder.WriteByte(' ')
	}

	builder.WriteString(levelStr)
	builder.WriteString("] ")
}

// appendCallerInfo formats and writes the caller information to the buffer.
func appendCallerInfo(builder *bytes.Buffer, file string, line int, funcName string) {
	if shortPath := filepath.Base(file); shortPath != "." {
		builder.WriteString(shortPath)
	} else {
		builder.WriteString(file)
	}

	builder.WriteByte(':')
	builder.WriteString(strconv.Itoa(line))
	builder.WriteByte(' ')
	builder.WriteString(funcName)
	builder.WriteByte(' ')
}

// appendFields formats and writes the fields to the buffer.
func appendFields(builder *bytes.Buffer, fields []hyperlogger.Field) {
	builder.WriteString(" {")

	for i, field := range fields {
		if i > 0 {
			builder.WriteString(", ")
		}

		builder.WriteString(field.Key)
		builder.WriteByte('=')
		appendFieldValue(builder, field.Value)
	}

	builder.WriteByte('}')
}

// appendFieldValue writes a field value to the buffer.
func appendFieldValue(builder *bytes.Buffer, value any) {
	// Optimize common field value types
	switch val := value.(type) {
	case string:
		builder.WriteString(val)
	case int:
		builder.WriteString(strconv.Itoa(val))
	case bool:
		if val {
			builder.WriteString("true")
		} else {
			builder.WriteString("false")
		}
	default:
		builder.WriteString(formatValue(val))
	}
}

// jsonEscapeString writes a properly escaped JSON string to the buffer.
// Uses direct byte operations instead of string concatenation for better performance.
func jsonEscapeString(buf *bytes.Buffer, target string) {
	buf.WriteByte('"')

	start := 0

	for i := range target {
		character := target[i]
		if needsEscaping(character) {
			// Write any pending bytes before this character
			if start < i {
				buf.WriteString(target[start:i])
			}

			writeEscapedChar(buf, character)

			start = i + 1
		}
	}

	// Write any remaining bytes
	if start < len(target) {
		buf.WriteString(target[start:])
	}

	buf.WriteByte('"')
}

// needsEscaping determines if a character needs special JSON escaping.
func needsEscaping(c byte) bool {
	switch c {
	case '"', '\\', '\b', '\f', '\n', '\r', '\t':
		return true
	default:
		return c < asciiControlStart || c > asciiControlEnd
	}
}

// writeEscapedChar writes the escaped version of a character to the buffer.
func writeEscapedChar(buf *bytes.Buffer, character byte) {
	switch character {
	case '"':
		buf.WriteString("\\\"")
	case '\\':
		buf.WriteString("\\\\")
	case '\b':
		buf.WriteString("\\b")
	case '\f':
		buf.WriteString("\\f")
	case '\n':
		buf.WriteString("\\n")
	case '\r':
		buf.WriteString("\\r")
	case '\t':
		buf.WriteString("\\t")
	default:
		// Control characters and others outside ASCII printable range
		fmt.Fprintf(buf, "\\u%04x", character)
	}
}

// formatJSONValue formats a value for JSON output.
// Specialized handling for common types to avoid any overhead.
//
//nolint:cyclop,funlen,revive // It's a long switch still readable.
func formatJSONValue(buf *bytes.Buffer, data any) {
	if data == nil {
		buf.WriteString("null")

		return
	}

	switch val := data.(type) {
	case string:
		jsonEscapeString(buf, val)
	case int:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case int8:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case int16:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case int32:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case int64:
		buf.WriteString(strconv.FormatInt(val, 10))
	case uint:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint8:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint16:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint64:
		buf.WriteString(strconv.FormatUint(val, 10))
	case float32:
		buf.WriteString(strconv.FormatFloat(float64(val), 'f', -1, 32))
	case float64:
		buf.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case error:
		jsonEscapeString(buf, val.Error())
	case time.Time:
		jsonEscapeString(buf, val.Format(time.RFC3339))
	case []byte:
		jsonEscapeString(buf, string(val))
	default: // Use fmt.Sprintf for other types
		jsonEscapeString(buf, fmt.Sprintf("%+v", val))
	}
}

// formatJSONOutput formats log entry as JSON.
// Optimized to minimize allocations and string operations.
func formatJSONOutput(builder *bytes.Buffer, entry *hyperlogger.Entry, config *hyperlogger.Config) {
	cfg := config
	if cfg == nil {
		cfg = &hyperlogger.Config{}
	}

	// Start JSON object
	builder.WriteString("{")

	// Add timestamp - use a single write operation with a precomputed string
	if !cfg.DisableTimestamp {
		timeFormat := cfg.TimeFormat
		if timeFormat == "" {
			timeFormat = time.RFC3339
		}

		timestamp := time.Now().Format(timeFormat)

		builder.WriteString(`"time":"`)
		builder.WriteString(timestamp)
		builder.WriteString(`",`)
	}

	// Add level - avoid string concatenation
	builder.WriteString(`"severity":"`)
	builder.WriteString(entry.Level.String())
	builder.WriteString(`",`)

	// Add caller if enabled
	if cfg.EnableCaller {
		file, line, funcName := getCaller(callerSkipLevel) // Adjust skip level as needed

		builder.WriteString(`"caller":"`)
		builder.WriteString(formatCallerInfo(file, line, funcName, true))
		builder.WriteString(`",`)
	}

	// Add message
	builder.WriteString(`"message":`)
	jsonEscapeString(builder, entry.Message)

	// Add fields more efficiently
	if len(entry.Fields) > 0 {
		builder.WriteByte(',')

		for i, field := range entry.Fields {
			if i > 0 {
				builder.WriteByte(',')
			}

			// Write field key as a JSON string
			builder.WriteByte('"')
			builder.WriteString(field.Key)
			builder.WriteString(`":`)

			// Format the value based on type
			formatJSONValue(builder, field.Value)
		}
	}

	// Close JSON object
	builder.WriteString("}\n")
}

func (a *Adapter) encodeEntry(entry *hyperlogger.Entry) ([]byte, *bytes.Buffer, error) {
	if entry == nil {
		return nil, nil, ewrap.New("entry cannot be nil")
	}

	if a.encoder == nil {
		return nil, nil, ewrap.Wrap(ErrEncoderNotFound, "no encoder configured")
	}

	size := a.encoder.EstimateSize(entry)
	if size <= 0 {
		size = predictBufferSize(a.config != nil && a.config.EnableJSON, len(entry.Message), len(entry.Fields))
	}

	buf := a.getBuffer(size)

	data, err := a.encoder.Encode(entry, a.config, buf)
	if err != nil {
		a.returnBuffer(buf)

		return nil, nil, err
	}

	return data, buf, nil
}

// log handles the common logging logic for all log levels.
func (a *Adapter) log(level hyperlogger.Level, msg string) {
	//nolint:gosec // The log levels can't be changed at runtime and cause integer overflow conversion.
	if level < hyperlogger.Level(a.level.Load()) {
		return // Skip logging if the level is below our configured level
	}

	if a.sampler != nil && !a.sampler.Allow(level) {
		return
	}

	// Create log entry with cloned base fields to avoid cross-request mutation.
	baseFields, storage := a.borrowFieldsSnapshot()
	defer a.releaseFieldsSnapshot(storage)

	entry := &hyperlogger.Entry{
		Level:   level,
		Message: msg,
		Fields:  baseFields,
	}

	// Add context values to fields
	entry.Fields = withContext(a.ctx, entry.Fields, a.config)

	// Attach stack trace for error-level logs when configured
	a.attachStackTrace(entry)

	// Execute hooks for this entry
	if a.hookRegistry != nil {
		a.processHooks(entry)
	}

	// Format and write the log entry
	encoded, buf, err := a.encodeEntry(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode log entry: %v\n", err)

		return
	}

	// Write to output, ensuring critical levels bypass async queues when necessary.
	if asyncWriter, ok := a.config.Output.(*output.AsyncWriter); ok {
		if level >= hyperlogger.ErrorLevel {
			_, err = asyncWriter.WriteCritical(encoded)
		} else {
			_, err = asyncWriter.Write(encoded)
		}
	} else {
		_, err = a.config.Output.Write(encoded)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
	}

	a.returnBuffer(buf)

	// If this is a fatal log, exit the program
	if level == hyperlogger.FatalLevel {
		a.exit(entry, 1)
	}
}

// exit handles cleanup and exits the program with the given code.
//
//nolint:revive
func (a *Adapter) exit(entry *hyperlogger.Entry, code int) {
	err := a.Sync()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sync logs before exit: %v\n", err)
		fmt.Fprintf(os.Stderr, "Original fatal log: %s\n", entry.Message)
	}

	os.Exit(code)
}

// processHooks executes registered hooks and handles any errors they produce.
func (a *Adapter) processHooks(entry *hyperlogger.Entry) {
	var hookErrs []error

	if a.hookRegistry != nil {
		hookErrs = append(hookErrs, a.hookRegistry.Dispatch(a.ctx, entry)...)
	}

	hookErrs = append(hookErrs, hyperlogger.FireRegisteredHooks(a.ctx, entry)...)

	// Log hook errors if any occurred
	for _, err := range hookErrs {
		if err != nil {
			a.logHookError(err)
		}
	}
}

// logHookError formats and logs an error that occurred during hook execution.
func (a *Adapter) logHookError(err error) {
	errMsg := fmt.Sprintf("Hook execution error: %v", err)

	// Create error entry
	errEntry := &hyperlogger.Entry{
		Level:   hyperlogger.ErrorLevel,
		Message: errMsg,
		Fields:  nil,
	}

	encoded, errBuf, encodeErr := a.encodeEntry(errEntry)
	if encodeErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode hook error: %v\n", encodeErr)

		return
	}

	// Write error log
	_, writeErr := a.config.Output.Write(encoded)
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to write error log: %v\n", writeErr)
	}

	a.returnBuffer(errBuf)
}

// predictBufferSize calculates a more accurate buffer size based on content.
func predictBufferSize(isJSON bool, msgLen int, fieldsLen int) int {
	var baseSize, fieldMultiplier int

	if isJSON {
		baseSize = jsonBaseSize
		fieldMultiplier = fieldOverhead
	} else {
		baseSize = consoleBaseSize
		fieldMultiplier = consoleFieldSize
	}

	// Calculate size based on message length and number of fields
	// Add extra capacity to avoid reallocations in most cases
	predictedSize := baseSize + msgLen + (fieldsLen * fieldMultiplier)

	// Round up to nearest power of 2 for optimal memory usage
	return nextPowerOfTwo(predictedSize)
}

// nextPowerOfTwo rounds up to the next power of 2.
//
//nolint:mnd,revive
func nextPowerOfTwo(val int) int {
	val--
	val |= val >> 1
	val |= val >> 2
	val |= val >> 4
	val |= val >> 8
	val |= val >> 16
	val++

	return val
}
