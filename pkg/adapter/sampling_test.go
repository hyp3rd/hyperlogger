package adapter

import (
	"sync"
	"sync/atomic"
	"testing"

	logger "github.com/hyp3rd/hyperlogger"
)

func TestLogSampler(t *testing.T) {
	cfg := logger.SamplingConfig{
		Enabled:    true,
		Initial:    2,
		Thereafter: 3,
	}

	sampler := newLogSampler(cfg)
	if sampler == nil {
		t.Fatal("expected sampler to be created")
	}

	// First two entries pass
	if allowed := sampler.Allow(logger.InfoLevel); !allowed {
		t.Fatalf("expected first entries to pass")
	}

	if allowed := sampler.Allow(logger.InfoLevel); !allowed {
		t.Fatalf("expected second entry to pass")
	}

	// Third entry skipped (info level)
	if sampler.Allow(logger.InfoLevel) {
		t.Fatalf("expected entry to be sampled out")
	}

	// Fourth entry still sampled out
	if sampler.Allow(logger.InfoLevel) {
		t.Fatalf("expected entry to remain sampled out")
	}

	// Fifth entry should pass (every third after initial)
	if !sampler.Allow(logger.InfoLevel) {
		t.Fatalf("expected entry to pass after sampling interval")
	}

	// Warn level should always pass
	if !sampler.Allow(logger.WarnLevel) {
		t.Fatalf("expected warn level to bypass sampling")
	}
}

func TestLogSamplerPerLevel(t *testing.T) {
	cfg := logger.SamplingConfig{
		Enabled:           true,
		PerLevelThreshold: true,
		Initial:           1,
		Thereafter:        2,
	}

	sampler := newLogSampler(cfg)
	if sampler == nil {
		t.Fatal("expected sampler to be created")
	}

	// Trace counter
	if !sampler.Allow(logger.TraceLevel) {
		t.Fatalf("expected first trace to pass")
	}
	if sampler.Allow(logger.TraceLevel) {
		t.Fatalf("expected second trace to be sampled")
	}

	// Debug counter separate from trace
	if !sampler.Allow(logger.DebugLevel) {
		t.Fatalf("expected first debug to pass")
	}
}

func TestLogSamplerConcurrency(t *testing.T) {
	cfg := logger.SamplingConfig{
		Enabled:    true,
		Initial:    0,
		Thereafter: 10,
	}

	sampler := newLogSampler(cfg)
	if sampler == nil {
		t.Fatal("expected sampler to be created")
	}

	const goroutines = 8
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	var allowed int32

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				if sampler.Allow(logger.InfoLevel) {
					atomic.AddInt32(&allowed, 1)
				}
			}
		}()
	}

	wg.Wait()

	if allowed == 0 {
		t.Fatalf("expected some entries to be logged")
	}
}
