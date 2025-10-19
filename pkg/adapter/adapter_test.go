package adapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/hyp3rd/ewrap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/internal/constants"
)

func TestNewAdapter_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      hyperlogger.Config
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil output",
			config:      hyperlogger.Config{},
			wantErr:     true,
			errContains: "output writer is required",
		},
		{
			name: "zero buffer size defaults to DefaultAsyncBufferSize",
			config: hyperlogger.Config{
				Output: &bytes.Buffer{},
			},
			wantErr: false,
		},
		{
			name: "custom buffer size",
			config: hyperlogger.Config{
				Output:          &bytes.Buffer{},
				AsyncBufferSize: 100,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewAdapter(context.Background(), tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, adapter)
		})
	}
}

func TestAdapter_WithContext(t *testing.T) {
	buf := &bytes.Buffer{}
	adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
		Output:     buf,
		EnableJSON: true,
		TimeFormat: time.RFC3339,
		Level:      hyperlogger.InfoLevel,
	})
	require.NoError(t, err)

	tests := []struct {
		name           string
		ctx            context.Context
		expectedFields int
	}{
		{
			name:           "nil context",
			ctx:            nil,
			expectedFields: 0,
		},
		{
			name:           "empty context",
			ctx:            context.Background(),
			expectedFields: 0,
		},
		{
			name:           "context with trace_id",
			ctx:            context.WithValue(context.Background(), constants.TraceKey{}, "123"),
			expectedFields: 1,
		},
		{
			name:           "context with request_id",
			ctx:            context.WithValue(context.Background(), constants.RequestKey{}, "456"),
			expectedFields: 1,
		},
		{
			name: "context with both ids",
			ctx: context.WithValue(
				context.WithValue(context.Background(), constants.TraceKey{}, "123"),
				constants.RequestKey{},
				"456",
			),
			expectedFields: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextLogger := adapter.WithContext(tt.ctx)
			require.NotNil(t, contextLogger)
		})
	}
}

func TestAdapter_WithError(t *testing.T) {
	buf := &bytes.Buffer{}
	adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
		Output:     buf,
		EnableJSON: true,
		Level:      hyperlogger.InfoLevel,
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		err           error
		expectedCount int
	}{
		{
			name:          "nil error",
			err:           nil,
			expectedCount: 0,
		},
		{
			name:          "simple error",
			err:           ewrap.New("test error"),
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorLogger := adapter.WithError(tt.err)
			require.NotNil(t, errorLogger)

			if tt.err == nil {
				assert.Equal(t, adapter, errorLogger)

				return
			}
		})
	}
}

func TestAdapter_LevelManagement(t *testing.T) {
	buf := &bytes.Buffer{}
	adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
		Output: buf,
		Level:  hyperlogger.InfoLevel,
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		setLevel  hyperlogger.Level
		logLevel  hyperlogger.Level
		logMsg    string
		shouldLog bool
	}{
		{
			name:      "info logs when level is info",
			setLevel:  hyperlogger.InfoLevel,
			logLevel:  hyperlogger.InfoLevel,
			logMsg:    "test info",
			shouldLog: true,
		},
		{
			name:      "debug doesn't log when level is info",
			setLevel:  hyperlogger.InfoLevel,
			logLevel:  hyperlogger.DebugLevel,
			logMsg:    "test debug",
			shouldLog: false,
		},
		{
			name:      "error logs when level is info",
			setLevel:  hyperlogger.InfoLevel,
			logLevel:  hyperlogger.ErrorLevel,
			logMsg:    "test error",
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			adapter.SetLevel(tt.setLevel)
			assert.Equal(t, tt.setLevel, adapter.GetLevel())

			switch tt.logLevel {
			case hyperlogger.InfoLevel:
				adapter.Info(tt.logMsg)
			case hyperlogger.DebugLevel:
				adapter.Debug(tt.logMsg)
			case hyperlogger.ErrorLevel:
				adapter.Error(tt.logMsg)
			}

			time.Sleep(10 * time.Millisecond)

			if tt.shouldLog {
				assert.Contains(t, buf.String(), tt.logMsg)
			} else {
				assert.Empty(t, buf.String())
			}
		})
	}
}

func TestAdapter_FormattedLogging(t *testing.T) {
	tests := []struct {
		name   string
		method string
		format string
		args   []any
	}{
		{
			name:   "Tracef",
			method: "Tracef",
			format: "trace message %s %d",
			args:   []any{"test", 123},
		},
		{
			name:   "Debugf",
			method: "Debugf",
			format: "debug message %s %d",
			args:   []any{"test", 123},
		},
		{
			name:   "Infof",
			method: "Infof",
			format: "info message %s %d",
			args:   []any{"test", 123},
		},
		{
			name:   "Warnf",
			method: "Warnf",
			format: "warn message %s %d",
			args:   []any{"test", 123},
		},
		{
			name:   "Errorf",
			method: "Errorf",
			format: "error message %s %d",
			args:   []any{"test", 123},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
				Output: buf,
				Level:  hyperlogger.TraceLevel,
			})
			require.NoError(t, err)

			expected := fmt.Sprintf(tt.format, tt.args...)

			switch tt.method {
			case "Tracef":
				adapter.Tracef(tt.format, tt.args...)
			case "Debugf":
				adapter.Debugf(tt.format, tt.args...)
			case "Infof":
				adapter.Infof(tt.format, tt.args...)
			case "Warnf":
				adapter.Warnf(tt.format, tt.args...)
			case "Errorf":
				adapter.Errorf(tt.format, tt.args...)
			}

			assert.Contains(t, buf.String(), expected)
		})
	}
}

func TestAdapter_WithField(t *testing.T) {
	buf := &bytes.Buffer{}
	adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
		Output:     buf,
		EnableJSON: true,
		Level:      hyperlogger.InfoLevel,
	})
	require.NoError(t, err)

	fieldLogger := adapter.WithField("test_key", "test_value")
	require.NotNil(t, fieldLogger)

	fieldLogger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "test_key")
	assert.Contains(t, output, "test_value")
	assert.Contains(t, output, "test message")
}

func TestAdapter_WithFields(t *testing.T) {
	tests := []struct {
		name     string
		fields   []hyperlogger.Field
		expected map[string]any
	}{
		{
			name:     "empty fields",
			fields:   []hyperlogger.Field{},
			expected: map[string]any{},
		},
		{
			name: "single field",
			fields: []hyperlogger.Field{
				{Key: "key1", Value: "value1"},
			},
			expected: map[string]any{"key1": "value1"},
		},
		{
			name: "multiple fields",
			fields: []hyperlogger.Field{
				{Key: "key1", Value: "value1"},
				{Key: "key2", Value: 42},
				{Key: "key3", Value: true},
			},
			expected: map[string]any{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
		},
		{
			name: "duplicate fields - last wins",
			fields: []hyperlogger.Field{
				{Key: "key1", Value: "first"},
				{Key: "key1", Value: "second"},
			},
			expected: map[string]any{"key1": "second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
				Output:     buf,
				EnableJSON: true,
				Level:      hyperlogger.InfoLevel,
			})
			require.NoError(t, err)

			fieldLogger := adapter.WithFields(tt.fields...)
			if len(tt.fields) == 0 {
				assert.Equal(t, adapter, fieldLogger)

				return
			}

			fieldLogger.Info("test message")

			output := buf.String()

			for key, value := range tt.expected {
				assert.Contains(t, output, key)
				assert.Contains(t, output, fmt.Sprintf("%v", value))
			}
		})
	}
}

func TestAdapter_GetConfig(t *testing.T) {
	config := hyperlogger.Config{
		Output:     &bytes.Buffer{},
		EnableJSON: true,
		Level:      hyperlogger.DebugLevel,
	}

	adapter, err := NewAdapter(context.Background(), config)
	require.NoError(t, err)

	returnedConfig := adapter.GetConfig()
	require.NotNil(t, returnedConfig)
	assert.Equal(t, config.EnableJSON, returnedConfig.EnableJSON)
	assert.Equal(t, config.Level, returnedConfig.Level)
}

func TestAdapter_BufferPooling(t *testing.T) {
	buf := &bytes.Buffer{}
	adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
		Output: buf,
		Level:  hyperlogger.InfoLevel,
	})
	require.NoError(t, err)

	adapterImpl := adapter.(*Adapter)

	tests := []struct {
		name         string
		requestSize  int
		expectedPool int
	}{
		{
			name:         "small buffer",
			requestSize:  500,
			expectedPool: 0, // smallBufferSize bucket
		},
		{
			name:         "medium buffer",
			requestSize:  2000,
			expectedPool: 1, // mediumBufferSize bucket
		},
		{
			name:         "large buffer",
			requestSize:  8000,
			expectedPool: 2, // largeBufferSize bucket
		},
		{
			name:         "xlarge buffer",
			requestSize:  20000,
			expectedPool: 3, // xlargeBufferSize bucket
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := adapterImpl.getBuffer(tt.requestSize)
			require.NotNil(t, buffer)
			assert.GreaterOrEqual(t, buffer.Cap(), tt.requestSize)

			// Test returning buffer
			adapterImpl.returnBuffer(buffer)
		})
	}
}

func TestAdapter_JSONFormatting(t *testing.T) {
	tests := []struct {
		name     string
		level    hyperlogger.Level
		message  string
		fields   []hyperlogger.Field
		contains []string
	}{
		{
			name:    "simple message",
			level:   hyperlogger.InfoLevel,
			message: "test message",
			contains: []string{
				`"message":"test message"`,
				`"severity":"INFO"`,
			},
		},
		{
			name:    "message with fields",
			level:   hyperlogger.ErrorLevel,
			message: "error occurred",
			fields: []hyperlogger.Field{
				{Key: "error_code", Value: 500},
				{Key: "user_id", Value: "12345"},
			},
			contains: []string{
				`"message":"error occurred"`,
				`"severity":"ERROR"`,
				`"error_code":500`,
				`"user_id":"12345"`,
			},
		},
		{
			name:    "special characters in message",
			level:   hyperlogger.WarnLevel,
			message: `message with "quotes" and \backslashes`,
			contains: []string{
				`"message":"message with \"quotes\" and \\backslashes"`,
				`"severity":"WARN"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
				Output:     buf,
				EnableJSON: true,
				Level:      hyperlogger.TraceLevel,
				TimeFormat: time.RFC3339,
			})
			require.NoError(t, err)

			if len(tt.fields) > 0 {
				adapter = adapter.WithFields(tt.fields...)
			}

			switch tt.level {
			case hyperlogger.InfoLevel:
				adapter.Info(tt.message)
			case hyperlogger.ErrorLevel:
				adapter.Error(tt.message)
			case hyperlogger.WarnLevel:
				adapter.Warn(tt.message)
			}

			output := buf.String()
			for _, contains := range tt.contains {
				assert.Contains(t, output, contains)
			}

			// Verify it's valid JSON
			err = ewrap.New("test") // Reset err

			lines := bytes.SplitSeq(buf.Bytes(), []byte("\n"))
			for line := range lines {
				if len(line) > 0 {
					err = ewrap.Newf("invalid JSON: %s", string(line))

					break
				}
			}
		})
	}
}

func TestAdapter_DisableTimestamp_TextOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := hyperlogger.Config{
		Output:           buf,
		EnableJSON:       false,
		Level:            hyperlogger.InfoLevel,
		DisableTimestamp: true,
		EnableCaller:     false,
		TimeFormat:       time.RFC3339,
	}

	adapter, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	adapter.Info("message without timestamp")

	output := buf.String()
	require.NotEmpty(t, output)

	firstRune, _ := utf8.DecodeRuneInString(output)
	assert.Equal(t, '[', firstRune, "expected log line to start with '[' when timestamp disabled")
}

func TestAdapter_DisableTimestamp_JSONOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := hyperlogger.Config{
		Output:           buf,
		EnableJSON:       true,
		Level:            hyperlogger.InfoLevel,
		DisableTimestamp: true,
		TimeFormat:       time.RFC3339,
	}

	adapter, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	adapter.Info("json without timestamp")

	output := buf.String()
	assert.NotContains(t, output, `"time":`, "expected JSON logs to omit time field when timestamp disabled")
}

func TestAdapter_AttachStackTrace(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := hyperlogger.Config{
		Output:           buf,
		EnableJSON:       true,
		Level:            hyperlogger.InfoLevel,
		EnableStackTrace: true,
	}

	adapter, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	adapter.Error("failure")

	output := buf.String()
	assert.Contains(t, output, `"stack":"`, "expected stack trace when EnableStackTrace is true")
	assert.Contains(t, output, `"message":"failure"`)

	buf.Reset()

	cfg.EnableStackTrace = false
	adapterNoStack, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	adapterNoStack.Error("failure no stack")
	assert.NotContains(t, buf.String(), `"stack":"`, "did not expect stack trace when disabled")
}

func TestAdapter_ContextDoesNotLeakBetweenLogs(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := hyperlogger.Config{
		Output:     buf,
		EnableJSON: true,
		Level:      hyperlogger.InfoLevel,
	}

	loggerWithCtx, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), constants.TraceKey{}, "trace-123")
	loggerWithCtx.WithContext(ctx).Info("with context")
	loggerWithCtx.Info("without context")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)

	assert.Contains(t, lines[0], `"trace_id":"trace-123"`)
	assert.NotContains(t, lines[1], `"trace_id"`, "context fields should not leak into subsequent logs")
}

func TestAdapter_CustomContextExtractors(t *testing.T) {
	buf := &bytes.Buffer{}
	extractorCalled := false

	type customContextKey struct{}

	cfg := hyperlogger.Config{
		Output:     buf,
		EnableJSON: true,
		Level:      hyperlogger.InfoLevel,
		ContextExtractors: []hyperlogger.ContextExtractor{
			func(ctx context.Context) []hyperlogger.Field {
				extractorCalled = true

				if val, ok := ctx.Value(customContextKey{}).(string); ok && val != "" {
					return []hyperlogger.Field{{Key: "custom", Value: val}}
				}

				return nil
			},
		},
	}

	loggerInstance, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), customContextKey{}, "value123")
	loggerInstance.WithContext(ctx).Info("custom message")

	require.True(t, extractorCalled)

	output := buf.String()
	assert.Contains(t, output, `"custom":"value123"`)
}

func TestAdapter_GlobalContextExtractors(t *testing.T) {
	defer hyperlogger.ClearContextExtractors()

	hyperlogger.ClearContextExtractors()
	hyperlogger.RegisterContextExtractor(func(ctx context.Context) []hyperlogger.Field {
		if v, ok := ctx.Value(constants.TraceKey{}).(string); ok {
			return []hyperlogger.Field{{Key: "trace", Value: v}}
		}

		return nil
	})

	buf := &bytes.Buffer{}
	cfg := hyperlogger.Config{}
	cfg.Output = buf
	cfg.EnableJSON = true
	cfg.Level = hyperlogger.DebugLevel

	loggerInstance, err := NewAdapter(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error creating adapter: %v", err)
	}

	ctx := context.WithValue(context.Background(), constants.TraceKey{}, "trace-xyz")
	loggerInstance.WithContext(ctx).Info("global context")

	output := buf.String()
	if !strings.Contains(output, `"trace":"trace-xyz"`) {
		t.Fatalf("expected output to contain trace field, got %s", output)
	}
}

func TestAdapter_AsyncMetricsHandler(t *testing.T) {
	hyperlogger.ClearAsyncMetricsHandlers()
	defer hyperlogger.ClearAsyncMetricsHandlers()

	var (
		mu              sync.Mutex
		localSnapshots  []hyperlogger.AsyncMetrics
		globalSnapshots []hyperlogger.AsyncMetrics
	)

	config := hyperlogger.Config{}
	config.Output = &bytes.Buffer{}
	config.Level = hyperlogger.InfoLevel
	config.EnableAsync = true
	config.AsyncBufferSize = 1
	config.AsyncOverflowStrategy = hyperlogger.AsyncOverflowDropNewest
	config.AsyncMetricsHandler = func(ctx context.Context, metrics hyperlogger.AsyncMetrics) {
		mu.Lock()

		localSnapshots = append(localSnapshots, metrics)

		mu.Unlock()
	}

	hyperlogger.RegisterAsyncMetricsHandler(func(ctx context.Context, metrics hyperlogger.AsyncMetrics) {
		mu.Lock()

		globalSnapshots = append(globalSnapshots, metrics)

		mu.Unlock()
	})

	loggerInstance, err := NewAdapter(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error creating adapter: %v", err)
	}

	loggerInstance.Info("first message")
	loggerInstance.Info("second message")

	time.Sleep(100 * time.Millisecond)

	_ = loggerInstance.Sync()

	mu.Lock()
	defer mu.Unlock()

	if len(localSnapshots) == 0 {
		t.Fatalf("expected async metrics handler to receive updates")
	}

	if len(globalSnapshots) == 0 {
		t.Fatalf("expected global async metrics handler to receive updates")
	}
}

func TestAdapter_CustomEncoderByName(t *testing.T) {
	buf := &bytes.Buffer{}
	name := "test-encoder-" + t.Name()
	encoder := &testEncoder{}

	registry := hyperlogger.NewEncoderRegistry()
	require.NoError(t, registry.Register(name, encoder))

	cfg := hyperlogger.Config{
		Output:          buf,
		Level:           hyperlogger.InfoLevel,
		EncoderName:     name,
		EncoderRegistry: registry,
	}

	loggerInstance, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	loggerInstance.Info("hello")

	assert.Contains(t, buf.String(), "encoded:hello")
	assert.Equal(t, 1, encoder.calls)
}

func TestAdapter_Sampling(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := hyperlogger.Config{
		Output:     buf,
		Level:      hyperlogger.InfoLevel,
		EnableJSON: true,
		Sampling: hyperlogger.SamplingConfig{
			Enabled:    true,
			Initial:    1,
			Thereafter: 2,
		},
	}

	loggerInstance, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	loggerInstance.Info("first")
	loggerInstance.Info("second")
	loggerInstance.Info("third")
	loggerInstance.Warn("always logged")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 logged lines (first, third, warn), got %d: %v", len(lines), lines)
	}

	if !strings.Contains(lines[0], "first") || !strings.Contains(lines[1], "third") {
		t.Fatalf("unexpected sampling behaviour: %v", lines)
	}

	if !strings.Contains(lines[2], "always logged") {
		t.Fatalf("warn level should bypass sampling: %v", lines[2])
	}
}

type testEncoder struct {
	calls int
}

func (e *testEncoder) Encode(entry *hyperlogger.Entry, _ *hyperlogger.Config, buf *bytes.Buffer) ([]byte, error) {
	e.calls++

	buf.Reset()
	buf.WriteString("encoded:" + entry.Message)

	return buf.Bytes(), nil
}

func (e *testEncoder) EstimateSize(entry *hyperlogger.Entry) int {
	if entry == nil {
		return 0
	}

	return len(entry.Message) + 8
}

func TestAdapter_GlobalHooks(t *testing.T) {
	defer hyperlogger.UnregisterAllHooks()

	hyperlogger.UnregisterAllHooks()

	var called atomic.Bool

	hyperlogger.RegisterHook(hyperlogger.InfoLevel, func(ctx context.Context, entry *hyperlogger.Entry) error {
		called.Store(true)

		if ctx == nil {
			t.Fatalf("expected context to be forwarded to hook")
		}

		return nil
	})

	buf := &bytes.Buffer{}
	loggerInstance, err := NewAdapter(context.Background(), hyperlogger.Config{
		Output: buf,
		Level:  hyperlogger.InfoLevel,
	})
	require.NoError(t, err)

	loggerInstance.Info("hooked")

	if !called.Load() {
		t.Fatalf("expected global hook to be triggered")
	}
}

func TestAdapter_FileOutputConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "app.log")
	buffer := &bytes.Buffer{}

	cfg := hyperlogger.Config{
		Output:      buffer,
		Level:       hyperlogger.InfoLevel,
		EnableAsync: false,
		File: hyperlogger.FileConfig{
			Path: logPath,
		},
	}

	loggerInstance, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	loggerInstance.Info("file-test-entry")
	require.NoError(t, loggerInstance.Sync())

	contents, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "file-test-entry")
	assert.Contains(t, buffer.String(), "file-test-entry")
}

func TestAdapter_LevelConcurrency(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	cfg := hyperlogger.Config{
		Output: buf,
		Level:  hyperlogger.InfoLevel,
	}

	loggerInstance, err := NewAdapter(context.Background(), cfg)
	require.NoError(t, err)

	const (
		goroutines = 8
		iterations = 500
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()

			for j := range iterations {
				level := hyperlogger.Level(j % int(hyperlogger.FatalLevel+1))
				loggerInstance.SetLevel(level)
				_ = loggerInstance.GetLevel()
			}
		}(i)
	}

	wg.Wait()

	finalLevel := loggerInstance.GetLevel()
	assert.True(t, finalLevel.IsValid())
}

func TestAdapter_Sync(t *testing.T) {
	tests := []struct {
		name        string
		enableAsync bool
		expectError bool
	}{
		{
			name:        "sync with regular buffer",
			enableAsync: false,
			expectError: false,
		},
		{
			name:        "sync with async enabled",
			enableAsync: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			config := hyperlogger.Config{
				Output:      buf,
				EnableAsync: tt.enableAsync,
				Level:       hyperlogger.InfoLevel,
			}

			adapter, err := NewAdapter(context.Background(), config)
			require.NoError(t, err)

			// Log something
			adapter.Info("test message")

			// Test sync
			err = adapter.Sync()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: "null",
		},
		{
			name:     "string",
			input:    "test",
			expected: "test",
		},
		{
			name:     "int",
			input:    42,
			expected: "42",
		},
		{
			name:     "bool true",
			input:    true,
			expected: "true",
		},
		{
			name:     "bool false",
			input:    false,
			expected: "false",
		},
		{
			name:     "time",
			input:    now,
			expected: now.Format(time.RFC3339),
		},
		{
			name:     "error",
			input:    ewrap.New("test error"),
			expected: "test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLevelColor(t *testing.T) {
	tests := []struct {
		name        string
		level       hyperlogger.Level
		expectColor bool
	}{
		{
			name:        "trace level",
			level:       hyperlogger.TraceLevel,
			expectColor: true,
		},
		{
			name:        "debug level",
			level:       hyperlogger.DebugLevel,
			expectColor: true,
		},
		{
			name:        "info level",
			level:       hyperlogger.InfoLevel,
			expectColor: true,
		},
		{
			name:        "warn level",
			level:       hyperlogger.WarnLevel,
			expectColor: true,
		},
		{
			name:        "error level",
			level:       hyperlogger.ErrorLevel,
			expectColor: true,
		},
		{
			name:        "fatal level",
			level:       hyperlogger.FatalLevel,
			expectColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, hasColor := getLevelColor(tt.level)
			assert.Equal(t, tt.expectColor, hasColor)
		})
	}
}

func TestPredictBufferSize(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		msgLen    int
		fieldsLen int
		expectMin int
	}{
		{
			name:      "json small",
			format:    "json",
			msgLen:    10,
			fieldsLen: 1,
			expectMin: jsonBaseSize,
		},
		{
			name:      "console small",
			format:    "console",
			msgLen:    10,
			fieldsLen: 1,
			expectMin: consoleBaseSize,
		},
		{
			name:      "json large",
			format:    "json",
			msgLen:    1000,
			fieldsLen: 10,
			expectMin: jsonBaseSize + 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := predictBufferSize(tt.format == "json", tt.msgLen, tt.fieldsLen)
			assert.GreaterOrEqual(t, size, tt.expectMin)
		})
	}
}

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{9, 16},
		{100, 128},
		{1000, 1024},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%d", tt.input), func(t *testing.T) {
			result := nextPowerOfTwo(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithContext_ContextValues(t *testing.T) {
	buf := &bytes.Buffer{}
	adapter, err := NewAdapter(context.Background(), hyperlogger.Config{
		Output:     buf,
		EnableJSON: true,
		Level:      hyperlogger.InfoLevel,
	})
	require.NoError(t, err)

	// Test with context containing namespace
	ctx := context.WithValue(context.Background(), constants.NamespaceKey{}, "test-namespace")
	contextLogger := adapter.WithContext(ctx)
	contextLogger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "namespace")
	assert.Contains(t, output, "test-namespace")
}

func TestExtractContextKeys(t *testing.T) {
	t.Run("nil context returns extras unchanged", func(t *testing.T) {
		initial := []hyperlogger.Field{{Key: "existing", Value: "value"}}
		result := extractContextKeys(context.Background(), initial)
		assert.Equal(t, initial, result)
	})

	t.Run("context with known keys adds fields", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), constants.TraceKey{}, "trace-123")
		ctx = context.WithValue(ctx, constants.RequestKey{}, "req-456")

		result := extractContextKeys(ctx, nil)
		require.Len(t, result, 2)

		values := make(map[string]any, len(result))
		for _, field := range result {
			values[field.Key] = field.Value
		}

		assert.Equal(t, "trace-123", values["trace_id"])
	})

	t.Run("non-string values are ignored", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), constants.TraceKey{}, 12345)

		result := extractContextKeys(ctx, nil)
		assert.Empty(t, result)
	})

	t.Run("fields appended to existing extras", func(t *testing.T) {
		extras := []hyperlogger.Field{{Key: "base", Value: "value"}}
		ctx := context.WithValue(context.Background(), constants.NamespaceKey{}, "ns-789")
		ctx = context.WithValue(ctx, constants.RequestKey{}, "req-456")

		result := extractContextKeys(ctx, extras)
		require.Len(t, result, 2+len(extras))

		values := make(map[string]any, len(result))
		for _, field := range result {
			values[field.Key] = field.Value
		}

		assert.Equal(t, "value", values["base"])
		assert.Equal(t, "ns-789", values["namespace"])
	})
}

func BenchmarkAdapterLogging(b *testing.B) {
	cases := []struct {
		name       string
		enableJSON bool
		withFields int
		multi      bool
	}{
		{
			name:       "TextLogging_NoFields",
			enableJSON: false,
			withFields: 0,
		},
		{
			name:       "TextLogging_5Fields",
			enableJSON: false,
			withFields: 5,
		},
		{
			name:       "JSONLogging_NoFields",
			enableJSON: true,
			withFields: 0,
		},
		{
			name:       "JSONLogging_5Fields",
			enableJSON: true,
			withFields: 5,
		},
		{
			name:       "TextLogging_MultiWriter",
			enableJSON: false,
			withFields: 2,
			multi:      true,
		},
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			buf := &bytes.Buffer{}

			var outputWriter io.Writer = buf
			if bc.multi {
				outputWriter = io.MultiWriter(buf, io.Discard)
			}

			cfg := hyperlogger.Config{
				Output:           outputWriter,
				EnableJSON:       bc.enableJSON,
				Level:            hyperlogger.InfoLevel,
				EnableCaller:     true,
				DisableTimestamp: false,
			}

			log, err := NewAdapter(context.Background(), cfg)
			if err != nil {
				b.Fatal(err)
			}

			// Add fields if needed
			if bc.withFields > 0 {
				fields := make([]hyperlogger.Field, bc.withFields)
				for i := range bc.withFields {
					fields[i] = hyperlogger.Field{
						Key:   fmt.Sprintf("key%d", i),
						Value: fmt.Sprintf("value%d", i),
					}
				}

				log = log.WithFields(fields...)
			}

			b.ResetTimer()

			for b.Loop() {
				log.Info("this is a benchmark test message")
			}

			// Ensure logs are processed
			if adapter, ok := log.(*Adapter); ok {
				adapter.Sync()
			}
		})
	}
}
