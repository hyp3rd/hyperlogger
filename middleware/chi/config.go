package chi

import "github.com/hyp3rd/hyperlogger"

// Config defines the configuration options for the Chi middleware.
type Config struct {
	Logger           hyperlogger.Logger
	IncludeHeaders   []string
	ContextExtractor func(r any) []hyperlogger.Field
	CaptureRequestID bool
	LatencyFieldName string
	StatusFieldName  string
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
