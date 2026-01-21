package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// Base provides common handler functionality including logging.
// Embed this struct in module handlers to get access to logging methods.
type Base struct {
	logger *zerolog.Logger
}

// NewBase creates a new Base handler with the provided logger.
func NewBase(logger *zerolog.Logger) Base {
	return Base{logger: logger}
}

// Logger returns the handler's logger.
func (h *Base) Logger() *zerolog.Logger {
	return h.logger
}

// LogRequest logs an incoming request with action context.
func (h *Base) LogRequest(c *fiber.Ctx, action string) {
	h.logger.Debug().
		Str("action", action).
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("ip", c.IP()).
		Msg("Request received")
}

// LogError logs an error with request context.
func (h *Base) LogError(c *fiber.Ctx, err error, msg string) {
	h.logger.Error().
		Err(err).
		Str("method", c.Method()).
		Str("path", c.Path()).
		Msg(msg)
}

// LogWarn logs a warning with request context.
func (h *Base) LogWarn(c *fiber.Ctx, msg string) {
	h.logger.Warn().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Msg(msg)
}

// LogInfo logs an info message with action context.
func (h *Base) LogInfo(c *fiber.Ctx, action, msg string) {
	h.logger.Info().
		Str("action", action).
		Str("method", c.Method()).
		Str("path", c.Path()).
		Msg(msg)
}

// LogDebug logs a debug message with optional fields.
func (h *Base) LogDebug(msg string, fields ...interface{}) {
	event := h.logger.Debug()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// GetTraceID extracts the trace ID from request context if available.
func (h *Base) GetTraceID(c *fiber.Ctx) string {
	if traceID, ok := c.Locals("traceID").(string); ok {
		return traceID
	}
	return c.Get("X-Trace-ID", "")
}
