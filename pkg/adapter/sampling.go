package adapter

import (
	"sync/atomic"

	logger "github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/pkg/security"
)

type logSampler struct {
	cfg           logger.SamplingConfig
	perLevel      bool
	initial       uint64
	thereafter    uint64
	globalCounter atomic.Uint64
	levelCounters [logger.FatalLevel + 1]*atomic.Uint64
}

func newLogSampler(cfg logger.SamplingConfig) *logSampler {
	if !cfg.Enabled {
		return nil
	}

	sanitized := sanitizeSamplingConfig(cfg)

	initial, err := security.SafeUint64FromInt(sanitized.Initial)
	if err != nil {
		return nil
	}

	thereafter, err := security.SafeUint64FromInt(sanitized.Thereafter)
	if err != nil {
		return nil
	}

	sampler := &logSampler{
		cfg:        sanitized,
		perLevel:   sanitized.PerLevelThreshold,
		initial:    initial,
		thereafter: thereafter,
	}

	if sampler.perLevel {
		for level := logger.TraceLevel; level <= logger.FatalLevel; level++ {
			sampler.levelCounters[level] = &atomic.Uint64{}
		}
	}

	return sampler
}

func sanitizeSamplingConfig(cfg logger.SamplingConfig) logger.SamplingConfig {
	if cfg.Initial <= 0 {
		cfg.Initial = logger.DefaultSamplingInitial
	}

	if cfg.Thereafter <= 0 {
		cfg.Thereafter = logger.DefaultSamplingThereafter
	}

	return cfg
}

func (s *logSampler) Allow(level logger.Level) bool {
	if s == nil {
		return true
	}

	// Never sample warnings or higher so that critical events are always logged.
	if level >= logger.WarnLevel {
		return true
	}

	counter := s.counter(level)
	currentCount := counter.Add(1)

	if currentCount <= s.initial {
		return true
	}

	if s.thereafter <= 1 {
		return true
	}

	return ((currentCount - s.initial) % s.thereafter) == 0
}

func (s *logSampler) counter(level logger.Level) *atomic.Uint64 {
	if s.perLevel {
		if int(level) < len(s.levelCounters) && s.levelCounters[level] != nil {
			return s.levelCounters[level]
		}
	}

	return &s.globalCounter
}
