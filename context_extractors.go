package hyperlogger

import (
	"context"
	"sync"
)

type contextExtractorRegistry struct {
	mu         sync.RWMutex
	extractors []ContextExtractor
}

//nolint:gochecknoglobals // package-wide registry ensures consistent extractor state.
var contextExtractorRegistryOnce = sync.OnceValue(func() *contextExtractorRegistry {
	return &contextExtractorRegistry{}
})

// RegisterContextExtractor adds a global context extractor that runs for every logger.
func RegisterContextExtractor(extractor ContextExtractor) {
	contextExtractorRegistryOnce().register(extractor)
}

// ClearContextExtractors removes all global context extractors.
func ClearContextExtractors() {
	contextExtractorRegistryOnce().reset()
}

// GlobalContextExtractors returns the currently registered global extractors.
func GlobalContextExtractors() []ContextExtractor {
	return contextExtractorRegistryOnce().snapshot()
}

// ApplyContextExtractors executes the provided extractors against the context.
func ApplyContextExtractors(ctx context.Context, extractors ...ContextExtractor) []Field {
	if len(extractors) == 0 {
		return nil
	}

	var fields []Field

	for _, extractor := range extractors {
		if extractor == nil {
			continue
		}

		if extracted := extractor(ctx); len(extracted) > 0 {
			fields = append(fields, extracted...)
		}
	}

	return fields
}

func (r *contextExtractorRegistry) register(extractor ContextExtractor) {
	if extractor == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.extractors = append(r.extractors, extractor)
}

func (r *contextExtractorRegistry) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.extractors = nil
}

func (r *contextExtractorRegistry) snapshot() []ContextExtractor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.extractors) == 0 {
		return nil
	}

	cloned := make([]ContextExtractor, len(r.extractors))
	copy(cloned, r.extractors)

	return cloned
}
