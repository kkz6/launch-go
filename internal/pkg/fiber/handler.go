package fiber

import (
	gofiber "github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/pkg/logger"
)

// Base provides common handler functionality including logging.
// Embed this struct in module handlers to get access to logging methods.
type BaseHandler struct {
	logger *zerolog.Logger
}

// NewBase creates a new Base handler with the provided logger.
func NewBaseHandler(log *zerolog.Logger) BaseHandler {
	return BaseHandler{logger: log}
}

// Logger returns the handler's logger.
func (h *BaseHandler) Logger() *zerolog.Logger {
	return h.logger
}

// LogRequest logs an incoming request with action context.
func (h *BaseHandler) LogRequest(c *gofiber.Ctx, action string) {
	h.logger.Debug().
		Str("action", action).
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("ip", c.IP()).
		Msg("Request received")
}

// LogError logs an error with request context.
func (h *BaseHandler) LogError(c *gofiber.Ctx, err error, msg string) {
	h.logger.Error().
		Err(err).
		Str("method", c.Method()).
		Str("path", c.Path()).
		Msg(msg)
}

// LogWarn logs a warning with request context.
func (h *BaseHandler) LogWarn(c *gofiber.Ctx, msg string) {
	h.logger.Warn().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Msg(msg)
}

// LogInfo logs an info message with action context.
func (h *BaseHandler) LogInfo(c *gofiber.Ctx, action, msg string) {
	h.logger.Info().
		Str("action", action).
		Str("method", c.Method()).
		Str("path", c.Path()).
		Msg(msg)
}

// LogDebug logs a debug message with optional fields.
func (h *BaseHandler) LogDebug(msg string, fields ...interface{}) {
	logger.Debug(h.logger, msg, fields...)
}

// GetTraceID extracts the trace ID from request context if available.
func (h *BaseHandler) GetTraceID(c *gofiber.Ctx) string {
	if traceID, ok := c.Locals("traceID").(string); ok {
		return traceID
	}
	return c.Get("X-Trace-ID", "")
}
