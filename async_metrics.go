package hyperlogger

import (
	"context"
	"sync"

	"github.com/hyp3rd/hyperlogger/internal/constants"
)

// AsyncMetrics represents health metrics emitted by the async writer.
type AsyncMetrics struct {
	Enqueued   uint64
	Processed  uint64
	Dropped    uint64
	WriteError uint64
	Retried    uint64
	QueueDepth int
	Bypassed   uint64
}

// AsyncMetricsHandler receives async writer metrics.
type AsyncMetricsHandler func(context.Context, AsyncMetrics)

//nolint:gochecknoglobals // async metrics use a package-level registry for global handlers.
var asyncMetricsRegistryOnce = sync.OnceValue(func() *asyncMetricsHandlerRegistry {
	return &asyncMetricsHandlerRegistry{}
})

// RegisterAsyncMetricsHandler adds a global handler invoked when async metrics are emitted.
func RegisterAsyncMetricsHandler(handler AsyncMetricsHandler) {
	if handler == nil {
		return
	}

	asyncMetricsRegistryOnce().register(handler)
}

// ClearAsyncMetricsHandlers removes all registered async metrics handlers.
func ClearAsyncMetricsHandlers() {
	asyncMetricsRegistryOnce().reset()
}

// EmitAsyncMetrics notifies global handlers with the provided metrics snapshot.
func EmitAsyncMetrics(ctx context.Context, metrics AsyncMetrics) {
	ctx, cancel := context.WithTimeout(ctx, constants.DefaultTimeout)
	defer cancel()

	asyncMetricsRegistryOnce().emit(ctx, metrics)
}

type asyncMetricsHandlerRegistry struct {
	mu       sync.RWMutex
	handlers []AsyncMetricsHandler
}

func (r *asyncMetricsHandlerRegistry) register(handler AsyncMetricsHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers = append(r.handlers, handler)
}

func (r *asyncMetricsHandlerRegistry) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers = nil
}

func (r *asyncMetricsHandlerRegistry) emit(ctx context.Context, metrics AsyncMetrics) {
	handlers := r.snapshot()
	for _, handler := range handlers {
		handler(ctx, metrics)
	}
}

func (r *asyncMetricsHandlerRegistry) snapshot() []AsyncMetricsHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.handlers) == 0 {
		return nil
	}

	clone := make([]AsyncMetricsHandler, len(r.handlers))
	copy(clone, r.handlers)

	return clone
}
