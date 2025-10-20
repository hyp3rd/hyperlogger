package hyperlogger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAsyncMetricsExporter(t *testing.T) {
	exporter := NewAsyncMetricsExporter()

	exporter.Observe(context.Background(), AsyncMetrics{
		Enqueued:   10,
		Processed:  8,
		Dropped:    2,
		WriteError: 1,
		Retried:    1,
		Bypassed:   3,
		QueueDepth: 4,
	})

	rec := httptest.NewRecorder()
	exporter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()

	for _, metric := range []string{
		"hyperlogger_async_enqueued_total 10",
		"hyperlogger_async_processed_total 8",
		"hyperlogger_async_dropped_total 2",
		"hyperlogger_async_write_errors_total 1",
		"hyperlogger_async_retried_total 1",
		"hyperlogger_async_bypassed_total 3",
		"hyperlogger_async_queue_depth 4",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("expected response to contain %q, got %q", metric, body)
		}
	}
}
