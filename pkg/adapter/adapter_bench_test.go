package adapter

import (
	"context"
	"testing"

	"github.com/hyp3rd/hyperlogger"
)

type benchWriter struct{}

func (benchWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (benchWriter) Sync() error {
	return nil
}

func (benchWriter) Close() error {
	return nil
}

func BenchmarkAdapterLoggingAllocations(b *testing.B) {
	testCases := []struct {
		name       string
		enableJSON bool
		fields     []hyperlogger.Field
	}{
		{name: "Console/NoFields"},
		{name: "Console/WithField", fields: []hyperlogger.Field{hyperlogger.Str("user", "bench")}},
		{name: "Console/WithFields", fields: []hyperlogger.Field{
			hyperlogger.Str("user", "bench"),
			hyperlogger.Str("trace_id", "123"),
		}},
		{name: "JSON/NoFields", enableJSON: true},
		{name: "JSON/WithField", enableJSON: true, fields: []hyperlogger.Field{hyperlogger.Str("user", "bench")}},
		{name: "JSON/WithFields", enableJSON: true, fields: []hyperlogger.Field{
			hyperlogger.Str("user", "bench"),
			hyperlogger.Str("trace_id", "123"),
		}},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			logger := newBenchmarkAdapter(b, tc.enableJSON)
			if len(tc.fields) > 0 {
				logger = logger.WithFields(tc.fields...)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				logger.Info("benchmark log line")
			}
		})
	}
}

func newBenchmarkAdapter(b *testing.B, enableJSON bool) hyperlogger.Logger {
	b.Helper()

	cfg := hyperlogger.Config{
		Output:           benchWriter{},
		Level:            hyperlogger.InfoLevel,
		EnableStackTrace: false,
		EnableCaller:     false,
		EnableJSON:       enableJSON,
		EnableAsync:      false,
		TimeFormat:       hyperlogger.DefaultTimeFormat,
	}

	logger, err := NewAdapter(context.Background(), cfg)
	if err != nil {
		b.Fatalf("failed to create adapter: %v", err)
	}

	return logger
}
