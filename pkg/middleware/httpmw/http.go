package httpmw

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/hyp3rd/hyperlogger/internal/constants"
)

const randomIDLength = 16

// Option configures the behaviour of the ContextMiddleware.
type Option func(*options)

type options struct {
	traceHeader    string
	requestHeader  string
	idGenerator    func() string
	generateIfMiss bool
}

// WithTraceHeader configures the header used to populate the trace id.
func WithTraceHeader(name string) Option {
	return func(o *options) {
		if name != "" {
			o.traceHeader = name
		}
	}
}

// WithRequestHeader configures the header used to populate the request id.
func WithRequestHeader(name string) Option {
	return func(o *options) {
		if name != "" {
			o.requestHeader = name
		}
	}
}

// WithIDGenerator provides a custom generator used when headers are missing.
func WithIDGenerator(fn func() string) Option {
	return func(o *options) {
		if fn != nil {
			o.idGenerator = fn
		}
	}
}

// WithGenerateMissingIDs instructs the middleware to create ids when headers are absent.
func WithGenerateMissingIDs(enable bool) Option {
	return func(o *options) {
		o.generateIfMiss = enable
	}
}

// ContextMiddleware enriches the request context with identifiers commonly used by the logger.
func ContextMiddleware(opts ...Option) func(http.Handler) http.Handler {
	cfg := options{
		traceHeader:    constants.TraceHeader,
		requestHeader:  constants.RequestHeader,
		idGenerator:    randomID,
		generateIfMiss: true,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if traceID := r.Header.Get(cfg.traceHeader); traceID != "" {
				ctx = contextWithValue(ctx, constants.TraceKey{}, traceID)
			} else if cfg.generateIfMiss {
				ctx = contextWithValue(ctx, constants.TraceKey{}, cfg.idGenerator())
			}

			if reqID := r.Header.Get(cfg.requestHeader); reqID != "" {
				ctx = contextWithValue(ctx, constants.RequestKey{}, reqID)
			} else if cfg.generateIfMiss {
				ctx = contextWithValue(ctx, constants.RequestKey{}, cfg.idGenerator())
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func contextWithValue(ctx context.Context, key any, value string) context.Context {
	if value == "" {
		return ctx
	}

	return context.WithValue(ctx, key, value)
}

func randomID() string {
	bytes := make([]byte, randomIDLength)

	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}

	return hex.EncodeToString(bytes)
}
