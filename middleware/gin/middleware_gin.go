//go:build gin_integration

package gin

import (
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hyp3rd/hyperlogger"
)

func Middleware(cfg Config) gin.HandlerFunc {
	cfg = cfg.withDefaults()

	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := make([]hyperlogger.Field, 0, 8+len(cfg.IncludeHeaders))
		fields = append(fields,
			hyperlogger.Str("method", c.Request.Method),
			hyperlogger.Str("path", c.FullPath()),
			hyperlogger.Str("host", c.Request.Host),
			hyperlogger.Str("client_ip", clientIP(c)),
			hyperlogger.Int(cfg.StatusFieldName, status),
			hyperlogger.Duration(cfg.LatencyFieldName, latency),
		)

		if cfg.CaptureRequestID {
			if id := c.Request.Header.Get("X-Request-ID"); id != "" {
				fields = append(fields, hyperlogger.Str("request_id", id))
			}
		}

		for _, header := range cfg.IncludeHeaders {
			if value := c.Request.Header.Get(header); value != "" {
				fields = append(fields, hyperlogger.Str("header_"+strings.ToLower(header), value))
			}
		}

		if extractor := cfg.ContextExtractor; extractor != nil {
			fields = append(fields, extractor(c)...)
		}

		logger := cfg.Logger.WithFields(fields...)

		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				logger.WithError(err).Error("gin request completed with error")
			}

			return
		}

		if status >= http.StatusInternalServerError {
			logger.Error("gin request returned server error")

			return
		}

		logger.Info("gin request completed")
	}
}

func Recovery(cfg Config) gin.HandlerFunc {
	cfg = cfg.withDefaults()

	return func(c *gin.Context) {
		if cfg.EnableRecovery {
			defer func() {
				if rec := recover(); rec != nil {
					fields := []hyperlogger.Field{
						hyperlogger.Any("panic", rec),
						hyperlogger.Str("path", c.FullPath()),
						hyperlogger.Str("method", c.Request.Method),
						hyperlogger.Str("stack", string(debug.Stack())),
					}

					cfg.Logger.WithFields(fields...).Error("gin panic recovered")
					c.AbortWithStatus(http.StatusInternalServerError)
				}
			}()
		}

		c.Next()
	}
}

func DefaultContextExtractor(ctx any) []hyperlogger.Field {
	c, ok := ctx.(*gin.Context)
	if !ok || c == nil {
		return nil
	}

	fields := make([]hyperlogger.Field, 0, 3)

	if route := c.FullPath(); route != "" {
		fields = append(fields, hyperlogger.Str("route", route))
	}

	if query := c.Request.URL.RawQuery; query != "" {
		fields = append(fields, hyperlogger.Str("query", query))
	}

	for _, param := range c.Params {
		fields = append(fields, hyperlogger.Str("param_"+param.Key, param.Value))
	}

	return fields
}

func clientIP(c *gin.Context) string {
	if ip := c.ClientIP(); ip != "" {
		return ip
	}

	return c.Request.RemoteAddr
}
