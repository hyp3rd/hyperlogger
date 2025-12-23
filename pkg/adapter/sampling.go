package adapter

import (
	"sync/atomic"

	"github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/pkg/security"
)

type samplingRule struct {
	enabled    bool
	initial    uint64
	thereafter uint64
	counter    atomic.Uint64
}

type logSampler struct {
	defaultRule *samplingRule
	levelRules  map[hyperlogger.Level]*samplingRule
}

func newLogSampler(cfg hyperlogger.SamplingConfig) *logSampler {
	if !cfg.Enabled {
		return nil
	}

	defaultRule, levelRules := sanitizeSamplingRules(cfg)
	if defaultRule == nil {
		return nil
	}

	return &logSampler{
		defaultRule: defaultRule,
		levelRules:  levelRules,
	}
}

func sanitizeSamplingRules(cfg hyperlogger.SamplingConfig) (*samplingRule, map[hyperlogger.Level]*samplingRule) {
	defaultRule, baseInitial, baseThereafter := buildDefaultSamplingRule(cfg)
	if defaultRule == nil {
		return nil, nil
	}

	levelRules := initializeLevelRules(cfg.PerLevelThreshold, defaultRule)
	applyRuleOverrides(levelRules, cfg.Rules, baseInitial, baseThereafter)

	return defaultRule, levelRules
}

func buildDefaultSamplingRule(cfg hyperlogger.SamplingConfig) (rule *samplingRule, initial, thereafter int) { //nolint:nonamedreturns
	initial = normalizeThreshold(cfg.Initial, hyperlogger.DefaultSamplingInitial)
	thereafter = normalizeThreshold(cfg.Thereafter, hyperlogger.DefaultSamplingThereafter)

	rule, err := newSamplingRule(true, initial, thereafter)
	if err != nil {
		return nil, 0, 0
	}

	return rule, initial, thereafter
}

func initializeLevelRules(perLevel bool, defaultRule *samplingRule) map[hyperlogger.Level]*samplingRule {
	levelRules := make(map[hyperlogger.Level]*samplingRule)

	if !perLevel {
		return levelRules
	}

	for level := hyperlogger.TraceLevel; level <= hyperlogger.FatalLevel; level++ {
		levelRules[level] = defaultRule.clone()
	}

	return levelRules
}

func applyRuleOverrides(
	levelRules map[hyperlogger.Level]*samplingRule,
	overrides map[hyperlogger.Level]hyperlogger.SamplingRule,
	baseInitial, baseThereafter int,
) {
	for level, cfg := range overrides {
		if !level.IsValid() {
			continue
		}

		rule := buildLevelRule(cfg, baseInitial, baseThereafter)
		levelRules[level] = rule
	}
}

func buildLevelRule(cfg hyperlogger.SamplingRule, baseInitial, baseThereafter int) *samplingRule {
	if !cfg.Enabled {
		return &samplingRule{enabled: false}
	}

	initial := normalizeThreshold(cfg.Initial, baseInitial)
	thereafter := normalizeThreshold(cfg.Thereafter, baseThereafter)

	rule, err := newSamplingRule(true, initial, thereafter)
	if err != nil {
		return &samplingRule{enabled: false}
	}

	return rule
}

func normalizeThreshold(value, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}

func newSamplingRule(enabled bool, initial, thereafter int) (*samplingRule, error) {
	initialUint, err := security.SafeUint64FromInt(initial)
	if err != nil {
		return nil, err
	}

	thereafterUint, err := security.SafeUint64FromInt(thereafter)
	if err != nil {
		return nil, err
	}

	return &samplingRule{
		enabled:    enabled,
		initial:    initialUint,
		thereafter: thereafterUint,
	}, nil
}

func (r *samplingRule) clone() *samplingRule {
	if r == nil {
		return nil
	}

	return &samplingRule{
		enabled:    r.enabled,
		initial:    r.initial,
		thereafter: r.thereafter,
	}
}

func (r *samplingRule) allow() bool {
	if r == nil || !r.enabled {
		return true
	}

	count := r.counter.Add(1)
	if count <= r.initial {
		return true
	}

	if r.thereafter <= 1 {
		return true
	}

	return ((count - r.initial) % r.thereafter) == 0
}

func (s *logSampler) Allow(level hyperlogger.Level) bool {
	if s == nil {
		return true
	}

	// Never sample warnings or higher so that critical events are always logged.
	if level >= hyperlogger.WarnLevel {
		return true
	}

	rule := s.defaultRule
	if specific, ok := s.levelRules[level]; ok {
		rule = specific
	}

	return rule.allow()
}
