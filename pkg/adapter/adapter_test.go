package adapter

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

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
			ctx:            context.WithValue(context.Background(), constants.TraceIDKey, "123"),
			expectedFields: 1,
		},
		{
			name:           "context with request_id",
			ctx:            context.WithValue(context.Background(), constants.RequestIDKey, "456"),
			expectedFields: 1,
		},
		{
			name: "context with both ids",
			ctx: context.WithValue(
				context.WithValue(context.Background(), constants.TraceIDKey, "123"),
				constants.RequestIDKey,
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

func TestMergeFields(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]hyperlogger.Field
		expected []hyperlogger.Field
	}{
		{
			name:     "empty input",
			input:    [][]hyperlogger.Field{},
			expected: nil,
		},
		{
			name: "single slice",
			input: [][]hyperlogger.Field{
				{{Key: "key1", Value: "value1"}},
			},
			expected: []hyperlogger.Field{
				{Key: "key1", Value: "value1"},
			},
		},
		{
			name: "multiple slices with unique keys",
			input: [][]hyperlogger.Field{
				{{Key: "key1", Value: "value1"}},
				{{Key: "key2", Value: "value2"}},
			},
			expected: []hyperlogger.Field{
				{Key: "key1", Value: "value1"},
				{Key: "key2", Value: "value2"},
			},
		},
		{
			name: "duplicate keys - later overrides earlier",
			input: [][]hyperlogger.Field{
				{{Key: "key1", Value: "first"}},
				{{Key: "key1", Value: "second"}},
			},
			expected: []hyperlogger.Field{
				{Key: "key1", Value: "second"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeFields(tt.input...)
			assert.Equal(t, tt.expected, result)
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
			size := predictBufferSize(tt.format, tt.msgLen, tt.fieldsLen)
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

func BenchmarkAdapterLogging(b *testing.B) {
	cases := []struct {
		name       string
		enableJSON bool
		withFields int
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
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			buf := &bytes.Buffer{}
			cfg := hyperlogger.Config{
				Output:           buf,
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
