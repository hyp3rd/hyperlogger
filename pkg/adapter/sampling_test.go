package adapter

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hyp3rd/hyperlogger"
)

func TestLogSampler(t *testing.T) {
	cfg := hyperlogger.SamplingConfig{
		Enabled:    true,
		Initial:    2,
		Thereafter: 3,
	}

	sampler := newLogSampler(cfg)
	if sampler == nil {
		t.Fatal("expected sampler to be created")
	}

	// First two entries pass
	if allowed := sampler.Allow(hyperlogger.InfoLevel); !allowed {
		t.Fatalf("expected first entries to pass")
	}

	if allowed := sampler.Allow(hyperlogger.InfoLevel); !allowed {
		t.Fatalf("expected second entry to pass")
	}

	// Third entry skipped (info level)
	if sampler.Allow(hyperlogger.InfoLevel) {
		t.Fatalf("expected entry to be sampled out")
	}

	// Fourth entry still sampled out
	if sampler.Allow(hyperlogger.InfoLevel) {
		t.Fatalf("expected entry to remain sampled out")
	}

	// Fifth entry should pass (every third after initial)
	if !sampler.Allow(hyperlogger.InfoLevel) {
		t.Fatalf("expected entry to pass after sampling interval")
	}

	// Warn level should always pass
	if !sampler.Allow(hyperlogger.WarnLevel) {
		t.Fatalf("expected warn level to bypass sampling")
	}
}

func TestLogSamplerPerLevel(t *testing.T) {
	cfg := hyperlogger.SamplingConfig{
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
	if !sampler.Allow(hyperlogger.TraceLevel) {
		t.Fatalf("expected first trace to pass")
	}
	if sampler.Allow(hyperlogger.TraceLevel) {
		t.Fatalf("expected second trace to be sampled")
	}

	// Debug counter separate from trace
	if !sampler.Allow(hyperlogger.DebugLevel) {
		t.Fatalf("expected first debug to pass")
	}
}

func TestLogSamplerRules(t *testing.T) {
	cfg := hyperlogger.SamplingConfig{
		Enabled:           true,
		Initial:           1,
		Thereafter:        5,
		PerLevelThreshold: false,
		Rules: map[hyperlogger.Level]hyperlogger.SamplingRule{
			hyperlogger.DebugLevel: {
				Enabled:    true,
				Initial:    2,
				Thereafter: 2,
			},
			hyperlogger.TraceLevel: {
				Enabled: false,
			},
		},
	}

	sampler := newLogSampler(cfg)
	if sampler == nil {
		t.Fatal("expected sampler to be created")
	}

	// Trace should bypass sampling entirely due to disabled rule.
	for i := 0; i < 10; i++ {
		if !sampler.Allow(hyperlogger.TraceLevel) {
			t.Fatalf("expected trace level to bypass sampling")
		}
	}

	// Debug uses per-level rule with different cadence.
	if !sampler.Allow(hyperlogger.DebugLevel) {
		t.Fatalf("expected first debug to pass")
	}
	if !sampler.Allow(hyperlogger.DebugLevel) {
		t.Fatalf("expected second debug to pass (initial=2)")
	}
	if sampler.Allow(hyperlogger.DebugLevel) {
		t.Fatalf("expected third debug to be sampled")
	}

	// Info falls back to default rule.
	if !sampler.Allow(hyperlogger.InfoLevel) {
		t.Fatalf("expected first info to pass")
	}
	if sampler.Allow(hyperlogger.InfoLevel) {
		t.Fatalf("expected subsequent info to follow default cadence")
	}
}

func TestLogSamplerConcurrency(t *testing.T) {
	cfg := hyperlogger.SamplingConfig{
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
				if sampler.Allow(hyperlogger.InfoLevel) {
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
