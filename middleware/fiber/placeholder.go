package fiber

import (
	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger"
)

// Handler defines the signature for a Fiber middleware handler function.
type Handler func(ctx any) error

// ErrIntegrationDisabled indicates that the fiber middleware was built without fiber support.
var ErrIntegrationDisabled = ewrap.New("fiber integration requires the 'fiber_integration' build tag")

// Middleware returns a stub fiber middleware when the fiber build tag is not provided.
func Middleware(cfg Config) Handler {
	cfg = cfg.withDefaults()

	return func(any) error {
		cfg.Logger.WithError(ErrIntegrationDisabled).Warn("fiber integration not enabled")

		return ErrIntegrationDisabled
	}
}

// DefaultContextExtractor is a no-op context extractor when fiber integration is not enabled.
func DefaultContextExtractor(any) []hyperlogger.Field {
	return nil
}
