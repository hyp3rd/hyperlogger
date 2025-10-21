package gin

import (
	"net/http"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger"
)

// ErrIntegrationDisabled indicates that the gin middleware was built without gin support.
var ErrIntegrationDisabled = ewrap.New("gin integration requires the 'gin_integration' build tag")

// Middleware returns a stub gin middleware when the gin build tag is not provided.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	cfg = cfg.withDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg.Logger.WithError(ErrIntegrationDisabled).Warn("gin integration not enabled")

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// Recovery returns a stub gin recovery middleware when the gin build tag is not provided.
func Recovery(cfg Config) func(http.Handler) http.Handler {
	cfg = cfg.withDefaults()

	return Middleware(cfg)
}

// DefaultContextExtractor is a no-op context extractor when gin integration is not enabled.
func DefaultContextExtractor(any) []hyperlogger.Field {
	return nil
}
