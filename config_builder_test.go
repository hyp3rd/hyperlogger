package hyperlogger

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewConfigBuilder(t *testing.T) {
	builder := NewConfigBuilder()

	if builder == nil {
		t.Fatal("NewConfigBuilder returned nil")
	}

	config := builder.Build()

	// Test default values
	if config.Output != os.Stdout {
		t.Error("Expected default output to be os.Stdout")
	}

	if config.Level != InfoLevel {
		t.Error("Expected default level to be InfoLevel")
	}

	if config.TimeFormat != time.RFC3339 {
		t.Error("Expected default time format to be RFC3339")
	}

	if !config.EnableCaller {
		t.Error("Expected caller to be enabled by default")
	}

	if !config.Color.Enable {
		t.Error("Expected colors to be enabled by default")
	}

	if config.Color.ForceTTY {
		t.Error("Expected ForceTTY to be false by default")
	}
}

func TestWithOutput(t *testing.T) {
	var buf bytes.Buffer

	config := NewConfigBuilder().WithOutput(&buf).Build()

	if config.Output != &buf {
		t.Error("WithOutput did not set output correctly")
	}
}

func TestWithConsoleOutput(t *testing.T) {
	config := NewConfigBuilder().WithConsoleOutput().Build()

	if config.Output != os.Stdout {
		t.Error("WithConsoleOutput did not set output to os.Stdout")
	}
}

func TestWithFileOutput(t *testing.T) {
	path := "/var/log/test.log"
	config := NewConfigBuilder().WithFileOutput(path).Build()

	if config.File.Path != path {
		t.Error("WithFileOutput did not set File.Path correctly")
	}
}

func TestWithLevel(t *testing.T) {
	config := NewConfigBuilder().WithLevel(ErrorLevel).Build()

	if config.Level != ErrorLevel {
		t.Error("WithLevel did not set level correctly")
	}
}

func TestWithDebugLevel(t *testing.T) {
	config := NewConfigBuilder().WithDebugLevel().Build()

	if config.Level != DebugLevel {
		t.Error("WithDebugLevel did not set level to DebugLevel")
	}
}

func TestWithInfoLevel(t *testing.T) {
	config := NewConfigBuilder().WithInfoLevel().Build()

	if config.Level != InfoLevel {
		t.Error("WithInfoLevel did not set level to InfoLevel")
	}
}

func TestWithTimeFormat(t *testing.T) {
	format := "2006-01-02 15:04:05"
	config := NewConfigBuilder().WithTimeFormat(format).Build()

	if config.TimeFormat != format {
		t.Error("WithTimeFormat did not set time format correctly")
	}
}

func TestWithNoTimestamp(t *testing.T) {
	config := NewConfigBuilder().WithNoTimestamp().Build()

	if !config.DisableTimestamp {
		t.Error("WithNoTimestamp did not disable timestamp")
	}
}

func TestWithCaller(t *testing.T) {
	config := NewConfigBuilder().WithCaller(false).Build()

	if config.EnableCaller {
		t.Error("WithCaller(false) did not disable caller")
	}

	config = NewConfigBuilder().WithCaller(true).Build()

	if !config.EnableCaller {
		t.Error("WithCaller(true) did not enable caller")
	}
}

func TestWithStackTrace(t *testing.T) {
	config := NewConfigBuilder().WithStackTrace(true).Build()

	if !config.EnableStackTrace {
		t.Error("WithStackTrace(true) did not enable stack trace")
	}
}

func TestWithJSONFormat(t *testing.T) {
	config := NewConfigBuilder().WithJSONFormat(true).Build()

	if !config.EnableJSON {
		t.Error("WithJSONFormat(true) did not enable JSON format")
	}
}

func TestWithColors(t *testing.T) {
	config := NewConfigBuilder().WithColors(false).Build()

	if config.Color.Enable {
		t.Error("WithColors(false) did not disable colors")
	}
}

func TestWithForceColors(t *testing.T) {
	config := NewConfigBuilder().WithForceColors(true).Build()

	if !config.Color.ForceTTY {
		t.Error("WithForceColors(true) did not force TTY")
	}
}

func TestWithAsyncBufferSize(t *testing.T) {
	size := 2000
	config := NewConfigBuilder().WithAsyncBufferSize(size).Build()

	if config.AsyncBufferSize != size {
		t.Error("WithAsyncBufferSize did not set buffer size correctly")
	}
}

func TestWithAsyncOverflowStrategy(t *testing.T) {
	config := NewConfigBuilder().WithAsyncOverflowStrategy(AsyncOverflowBlock).Build()

	if config.AsyncOverflowStrategy != AsyncOverflowBlock {
		t.Error("WithAsyncOverflowStrategy did not set strategy correctly")
	}
}

func TestWithAsyncDropHandler(t *testing.T) {
	called := false
	handler := func([]byte) { called = true }

	config := NewConfigBuilder().WithAsyncDropHandler(handler).Build()

	if config.AsyncDropHandler == nil {
		t.Fatal("expected drop handler to be set")
	}

	config.AsyncDropHandler([]byte("test"))

	if !called {
		t.Error("drop handler was not invoked")
	}
}

func TestWithAsyncMetricsHandler(t *testing.T) {
	called := false
	config := NewConfigBuilder().WithAsyncMetricsHandler(func(ctx context.Context, metrics AsyncMetrics) {
		called = true
	}).Build()

	if config.AsyncMetricsHandler == nil {
		t.Fatal("expected async metrics handler to be set")
	}

	config.AsyncMetricsHandler(context.Background(), AsyncMetrics{})
	if !called {
		t.Fatal("expected handler to be invoked")
	}
}

func TestWithContextExtractor(t *testing.T) {
	var saw bool
	type ctxKey struct{}

	extractor := func(ctx context.Context) []Field {
		saw = true
		if val, ok := ctx.Value(ctxKey{}).(string); ok && val != "" {
			return []Field{{Key: "example", Value: val}}
		}

		return nil
	}

	config := NewConfigBuilder().WithContextExtractor(extractor).Build()

	if len(config.ContextExtractors) != 1 {
		t.Fatalf("expected 1 extractor, got %d", len(config.ContextExtractors))
	}

	ctx := context.WithValue(context.Background(), ctxKey{}, "value")
	fields := config.ContextExtractors[0](ctx)
	if !saw || len(fields) != 1 || fields[0].Value != "value" {
		t.Fatalf("context extractor not invoked correctly: %+v", fields)
	}
}

func TestWithEncoderAndEncoderName(t *testing.T) {
	encoder := &mockEncoder{}
	registry := NewEncoderRegistry()
	require.NoError(t, registry.Register("custom", encoder))

	cfg := NewConfigBuilder().
		WithEncoderName("custom").
		WithEncoderRegistry(registry).
		WithEncoder(encoder).
		Build()

	if cfg.Encoder != encoder {
		t.Fatalf("expected encoder to be set")
	}

	if cfg.EncoderName != "custom" {
		t.Fatalf("expected encoder name to be set")
	}
}

func TestWithField(t *testing.T) {
	config := NewConfigBuilder().WithField("test", "value").Build()

	if len(config.AdditionalFields) != 1 {
		t.Error("WithField did not add field")
	}

	if config.AdditionalFields[0].Key != "test" {
		t.Error("WithField did not set field key correctly")
	}

	if config.AdditionalFields[0].Value != "value" {
		t.Error("WithField did not set field value correctly")
	}
}

func TestWithFields(t *testing.T) {
	fields := []Field{
		{Key: "field1", Value: "value1"},
		{Key: "field2", Value: "value2"},
	}
	config := NewConfigBuilder().WithFields(fields).Build()

	if len(config.AdditionalFields) != 2 {
		t.Error("WithFields did not add all fields")
	}
}

type mockEncoder struct{}

func (m *mockEncoder) Encode(*Entry, *Config, *bytes.Buffer) ([]byte, error) {
	return []byte("encoded"), nil
}

func (m *mockEncoder) EstimateSize(*Entry) int {
	return 0
}

func TestWithFileRotation(t *testing.T) {
	maxSize := int64(1024 * 1024 * 100)
	compress := true
	config := NewConfigBuilder().WithFileRotation(maxSize, compress).Build()

	if config.File.MaxSizeBytes != maxSize {
		t.Error("WithFileRotation did not set File.MaxSizeBytes correctly")
	}

	if config.File.Compress != compress {
		t.Error("WithFileRotation did not set File.Compress correctly")
	}

	if config.FileMaxSize != maxSize {
		t.Error("WithFileRotation did not set FileMaxSize correctly")
	}

	if config.FileCompress != compress {
		t.Error("WithFileRotation did not set FileCompress correctly")
	}
}

func TestWithSampling(t *testing.T) {
	config := NewConfigBuilder().WithSampling(true, 100, 10, true).Build()

	if !config.Sampling.Enabled {
		t.Error("WithSampling did not enable sampling")
	}

	if config.Sampling.Initial != 100 {
		t.Error("WithSampling did not set initial correctly")
	}

	if config.Sampling.Thereafter != 10 {
		t.Error("WithSampling did not set thereafter correctly")
	}

	if !config.Sampling.PerLevelThreshold {
		t.Error("WithSampling did not set PerLevelThreshold correctly")
	}
}

func TestWithSamplingRule(t *testing.T) {
	rule := SamplingRule{Enabled: true, Initial: 5, Thereafter: 3}
	config := NewConfigBuilder().WithSampling(true, 100, 10, false).
		WithSamplingRule(InfoLevel, rule).
		Build()

	if len(config.Sampling.Rules) != 1 {
		t.Fatalf("expected 1 sampling rule, got %d", len(config.Sampling.Rules))
	}

	stored, ok := config.Sampling.Rules[InfoLevel]
	if !ok {
		t.Fatalf("expected rule for info level")
	}

	if stored.Initial != 5 || stored.Thereafter != 3 || !stored.Enabled {
		t.Fatalf("unexpected rule stored: %+v", stored)
	}
}

func TestWithHook(t *testing.T) {
	hook := &mockHook{levels: []Level{ErrorLevel}}
	config := NewConfigBuilder().WithHook("test", hook).Build()

	if len(config.Hooks) != 1 {
		t.Error("WithHook did not add hook")
	}

	if config.Hooks[0].Name != "test" {
		t.Error("WithHook did not set hook name correctly")
	}

	if config.Hooks[0].Hook != hook {
		t.Error("WithHook did not set hook correctly")
	}
}

func TestWithFileCompression(t *testing.T) {
	level := 5
	config := NewConfigBuilder().WithFileCompression(level).Build()

	if config.File.CompressionLevel != level {
		t.Error("WithFileCompression did not set compression level correctly")
	}
}

func TestWithFileRetention(t *testing.T) {
	maxAge := 7
	maxFiles := 10
	config := NewConfigBuilder().WithFileRetention(maxAge, maxFiles).Build()

	if config.File.MaxAge != maxAge {
		t.Error("WithFileRetention did not set MaxAge correctly")
	}

	if config.File.MaxBackups != maxFiles {
		t.Error("WithFileRetention did not set MaxBackups correctly")
	}
}

func TestWithEnableAsync(t *testing.T) {
	config := NewConfigBuilder().WithEnableAsync(true).Build()

	if !config.EnableAsync {
		t.Error("WithEnableAsync(true) did not enable async")
	}
}

func TestWithLocalDefaults(t *testing.T) {
	config := NewConfigBuilder().WithLocalDefaults().Build()

	if config.Level != DebugLevel {
		t.Error("WithLocalDefaults did not set debug level")
	}

	if !config.EnableCaller {
		t.Error("WithLocalDefaults did not enable caller")
	}

	if !config.Color.Enable {
		t.Error("WithLocalDefaults did not enable colors")
	}

	if config.EnableJSON {
		t.Error("WithLocalDefaults should disable JSON format")
	}

	if !config.EnableStackTrace {
		t.Error("WithLocalDefaults did not enable stack trace")
	}
}

func TestWithDevelopmentDefaults(t *testing.T) {
	config := NewConfigBuilder().WithDevelopmentDefaults().Build()

	if config.Level != DebugLevel {
		t.Error("WithDevelopmentDefaults did not set debug level")
	}

	if !config.EnableCaller {
		t.Error("WithDevelopmentDefaults did not enable caller")
	}

	if config.Color.Enable {
		t.Error("WithDevelopmentDefaults should disable colors")
	}

	if !config.EnableJSON {
		t.Error("WithDevelopmentDefaults did not enable JSON format")
	}

	if !config.EnableStackTrace {
		t.Error("WithDevelopmentDefaults did not enable stack trace")
	}
}

func TestWithProductionDefaults(t *testing.T) {
	config := NewConfigBuilder().WithProductionDefaults().Build()

	if config.Level != InfoLevel {
		t.Error("WithProductionDefaults did not set info level")
	}

	if config.EnableCaller {
		t.Error("WithProductionDefaults should disable caller")
	}

	if config.Color.Enable {
		t.Error("WithProductionDefaults should disable colors")
	}

	if !config.EnableJSON {
		t.Error("WithProductionDefaults did not enable JSON format")
	}
}

func TestBuildReturnsCopy(t *testing.T) {
	builder := NewConfigBuilder()
	config1 := builder.Build()
	config2 := builder.Build()

	// Modify one config and ensure the other is not affected
	config1.Level = ErrorLevel

	if config2.Level == ErrorLevel {
		t.Error("Build() should return a copy, not a reference")
	}
}

func TestChaining(t *testing.T) {
	config := NewConfigBuilder().
		WithLevel(WarnLevel).
		WithJSONFormat(true).
		WithColors(false).
		WithCaller(false).
		Build()

	if config.Level != WarnLevel {
		t.Error("Chained WithLevel did not work")
	}

	if !config.EnableJSON {
		t.Error("Chained WithJSONFormat did not work")
	}

	if config.Color.Enable {
		t.Error("Chained WithColors did not work")
	}

	if config.EnableCaller {
		t.Error("Chained WithCaller did not work")
	}
}
