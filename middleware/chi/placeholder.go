package chi

import (
	"net/http"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger"
)

// ErrIntegrationDisabled indicates that the chi middleware was built without chi support.
var ErrIntegrationDisabled = ewrap.New("chi integration requires the 'chi_integration' build tag")

// Middleware returns a stub chi middleware when the chi build tag is not provided.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	cfg = cfg.withDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg.Logger.WithError(ErrIntegrationDisabled).Warn("chi integration not enabled")

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// DefaultContextExtractor is a no-op context extractor when chi integration is not enabled.
func DefaultContextExtractor(_ any) []hyperlogger.Field {
	return nil
}
