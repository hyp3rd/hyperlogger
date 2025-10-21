//go:build chi_integration

package chi

import (
	"net/http"
	"strings"
	"time"

	chi "github.com/go-chi/chi/v5"

	"github.com/hyp3rd/hyperlogger"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Status() int {
	if rw.status == 0 {
		return http.StatusOK
	}

	return rw.status
}

// Middleware returns a chi middleware that logs HTTP requests using hyperlogger.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	cfg = cfg.withDefaults()

	return func(next http.Handler) http.Handler {
		if next == nil {
			next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &responseWriter{ResponseWriter: w}
			start := time.Now()

			next.ServeHTTP(recorder, r)

			latency := time.Since(start)
			status := recorder.Status()

			fields := make([]hyperlogger.Field, 0, 6+len(cfg.IncludeHeaders))
			fields = append(fields,
				hyperlogger.Str("method", r.Method),
				hyperlogger.Str("path", r.URL.Path),
				hyperlogger.Str("host", r.Host),
				hyperlogger.Str("remote_addr", remoteAddr(r)),
				hyperlogger.Int(cfg.StatusFieldName, status),
				hyperlogger.Duration(cfg.LatencyFieldName, latency),
			)

			if cfg.CaptureRequestID {
				if id := r.Header.Get("X-Request-ID"); id != "" {
					fields = append(fields, hyperlogger.Str("request_id", id))
				}
			}

			for _, header := range cfg.IncludeHeaders {
				if value := r.Header.Get(header); value != "" {
					fields = append(fields, hyperlogger.Str("header_"+strings.ToLower(header), value))
				}
			}

			if extractor := cfg.ContextExtractor; extractor != nil {
				fields = append(fields, extractor(r)...)
			}

			logger := cfg.Logger.WithFields(fields...)

			if status >= http.StatusInternalServerError {
				logger.Error("chi request returned server error")

				return
			}

			logger.Info("chi request completed")
		})
	}
}

// DefaultContextExtractor extracts common fields from a chi *http.Request context.
func DefaultContextExtractor(ctx any) []hyperlogger.Field {
	r, ok := ctx.(*http.Request)
	if !ok || r == nil {
		return nil
	}

	fields := make([]hyperlogger.Field, 0, 3)

	if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
		if pattern := routeCtx.RoutePattern(); pattern != "" {
			fields = append(fields, hyperlogger.Str("route", pattern))
		}

		if len(routeCtx.URLParams.Keys) > 0 {
			for i, key := range routeCtx.URLParams.Keys {
				fields = append(fields, hyperlogger.Str("param_"+key, routeCtx.URLParams.Values[i]))
			}
		}
	}

	if query := r.URL.RawQuery; query != "" {
		fields = append(fields, hyperlogger.Str("query", query))
	}

	return fields
}

func remoteAddr(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	return r.RemoteAddr
}
