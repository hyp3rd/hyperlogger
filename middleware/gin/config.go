package gin

import "github.com/hyp3rd/hyperlogger"

// Config defines the configuration options for the Gin middleware.
type Config struct {
	Logger           hyperlogger.Logger
	IncludeHeaders   []string
	ContextExtractor func(c any) []hyperlogger.Field
	CaptureRequestID bool
	LatencyFieldName string
	StatusFieldName  string
	EnableRecovery   bool
}

func (c Config) withDefaults() Config {
	if c.Logger == nil {
		c.Logger = hyperlogger.NewNoop()
	}

	if c.ContextExtractor == nil {
		c.ContextExtractor = DefaultContextExtractor
	}

	if c.LatencyFieldName == "" {
		c.LatencyFieldName = "latency"
	}

	if c.StatusFieldName == "" {
		c.StatusFieldName = "status"
	}

	return c
}
