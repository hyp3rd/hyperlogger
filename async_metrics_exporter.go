package hyperlogger

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
)

// AsyncMetricsExporter exposes async writer metrics via a Prometheus-style HTTP handler.
// Register the Observe method using RegisterAsyncMetricsHandler to begin collecting data.
type AsyncMetricsExporter struct {
	enqueued   atomic.Uint64
	processed  atomic.Uint64
	dropped    atomic.Uint64
	writeError atomic.Uint64
	retried    atomic.Uint64
	bypassed   atomic.Uint64
	queueDepth atomic.Int64
}

// NewAsyncMetricsExporter creates a new exporter instance.
func NewAsyncMetricsExporter() *AsyncMetricsExporter {
	return &AsyncMetricsExporter{}
}

// Observe can be registered with RegisterAsyncMetricsHandler to record async metrics snapshots.
func (e *AsyncMetricsExporter) Observe(_ context.Context, metrics AsyncMetrics) {
	e.enqueued.Store(metrics.Enqueued)
	e.processed.Store(metrics.Processed)
	e.dropped.Store(metrics.Dropped)
	e.writeError.Store(metrics.WriteError)
	e.retried.Store(metrics.Retried)
	e.bypassed.Store(metrics.Bypassed)
	e.queueDepth.Store(int64(metrics.QueueDepth))
}

// ServeHTTP renders the metrics using Prometheus exposition format.
func (e *AsyncMetricsExporter) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintln(w, "# HELP hyperlogger_async_enqueued_total Total log entries enqueued")
	fmt.Fprintln(w, "# TYPE hyperlogger_async_enqueued_total counter")
	fmt.Fprintf(w, "hyperlogger_async_enqueued_total %d\n", e.enqueued.Load())

	fmt.Fprintln(w, "# HELP hyperlogger_async_processed_total Total log entries processed")
	fmt.Fprintln(w, "# TYPE hyperlogger_async_processed_total counter")
	fmt.Fprintf(w, "hyperlogger_async_processed_total %d\n", e.processed.Load())

	fmt.Fprintln(w, "# HELP hyperlogger_async_dropped_total Total log entries dropped")
	fmt.Fprintln(w, "# TYPE hyperlogger_async_dropped_total counter")
	fmt.Fprintf(w, "hyperlogger_async_dropped_total %d\n", e.dropped.Load())

	fmt.Fprintln(w, "# HELP hyperlogger_async_write_errors_total Total async write errors")
	fmt.Fprintln(w, "# TYPE hyperlogger_async_write_errors_total counter")
	fmt.Fprintf(w, "hyperlogger_async_write_errors_total %d\n", e.writeError.Load())

	fmt.Fprintln(w, "# HELP hyperlogger_async_retried_total Total async write retries")
	fmt.Fprintln(w, "# TYPE hyperlogger_async_retried_total counter")
	fmt.Fprintf(w, "hyperlogger_async_retried_total %d\n", e.retried.Load())

	fmt.Fprintln(w, "# HELP hyperlogger_async_bypassed_total Total log entries written synchronously")
	fmt.Fprintln(w, "# TYPE hyperlogger_async_bypassed_total counter")
	fmt.Fprintf(w, "hyperlogger_async_bypassed_total %d\n", e.bypassed.Load())

	fmt.Fprintln(w, "# HELP hyperlogger_async_queue_depth Current async queue depth")
	fmt.Fprintln(w, "# TYPE hyperlogger_async_queue_depth gauge")
	fmt.Fprintf(w, "hyperlogger_async_queue_depth %d\n", e.queueDepth.Load())
}
