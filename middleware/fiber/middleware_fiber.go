//go:build fiber_integration

package fiber

import (
	"strconv"
	"time"

	fiber "github.com/gofiber/fiber/v3"

	"github.com/hyp3rd/hyperlogger"
)

// Handler is an alias for the Fiber middleware handler function type.
type Handler = fiber.Handler

// Middleware returns a fiber middleware that logs HTTP requests using hyperlogger.
func Middleware(cfg Config) fiber.Handler {
	cfg = cfg.withDefaults()

	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		status := c.Response().StatusCode()

		fields := make([]hyperlogger.Field, 0, 6+len(cfg.IncludeHeaders))
		fields = append(fields,
			hyperlogger.Str("method", c.Method()),
			hyperlogger.Str("path", string(c.Request().URI().Path())),
			hyperlogger.Str("ip", c.IP()),
			hyperlogger.Str(cfg.StatusFieldName, fiberStatusText(status)),
			hyperlogger.Duration(cfg.LatencyFieldName, latency),
		)

		if cfg.CaptureRequestID {
			if id := c.GetRespHeader("X-Request-ID"); id != "" {
				fields = append(fields, hyperlogger.Str("request_id", id))
			}
		}

		for _, header := range cfg.IncludeHeaders {
			if value := c.Get(header); value != "" {
				fields = append(fields, hyperlogger.Str("header_"+header, value))
			}
		}

		if extractor := cfg.ContextExtractor; extractor != nil {
			fields = append(fields, extractor(c)...)
		}

		logger := cfg.Logger.WithFields(fields...)

		if err != nil {
			logger.WithError(err).Error("fiber request completed with error")

			return err
		}

		if status >= fiber.StatusInternalServerError {
			logger.Error("fiber request returned server error")

			return nil
		}

		logger.Info("fiber request completed")

		return nil
	}
}

// DefaultContextExtractor extracts common fields from a fiber *fiber.Ctx context.
func DefaultContextExtractor(ctx any) []hyperlogger.Field {
	c, ok := ctx.(*fiber.Ctx)
	if !ok || c == nil {
		return nil
	}

	fields := make([]hyperlogger.Field, 0, 2)

	if route := c.Route(); route != nil {
		fields = append(fields, hyperlogger.Str("route", route.Path))
	}

	if query := string(c.Request().URI().QueryString()); query != "" {
		fields = append(fields, hyperlogger.Str("query", query))
	}

	return fields
}

func fiberStatusText(status int) string {
	if text := fiber.StatusMessage(status); text != "" {
		return text
	}

	return strconv.Itoa(status)
}
