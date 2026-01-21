package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

const (
	// TraceIDKey is the key used to store trace ID in fiber context locals
	TraceIDKey = "traceID"

	// TraceIDHeader is the HTTP header name for trace ID
	TraceIDHeader = "X-Trace-ID"

	// RequestIDHeader is an alternative header name used by some services
	RequestIDHeader = "X-Request-ID"
)

// Trace adds a trace ID to the request context.
// If a trace ID is provided in the request headers, it will be used.
// Otherwise, a new ULID will be generated.
//
// The trace ID is:
// - Stored in fiber context locals under TraceIDKey
// - Added to response headers as X-Trace-ID
//
// Usage:
//
//	app.Use(middleware.Trace())
//
//	// In handlers:
//	traceID := middleware.GetTraceID(c)
func Trace() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check for existing trace ID in headers
		traceID := c.Get(TraceIDHeader)
		if traceID == "" {
			traceID = c.Get(RequestIDHeader)
		}

		// Generate new trace ID if not provided
		if traceID == "" {
			traceID = ulid.Make().String()
		}

		// Store in context locals
		c.Locals(TraceIDKey, traceID)

		// Add to response headers
		c.Set(TraceIDHeader, traceID)

		return c.Next()
	}
}

// GetTraceID extracts the trace ID from the fiber context.
// Returns empty string if no trace ID is set.
func GetTraceID(c *fiber.Ctx) string {
	if v, ok := c.Locals(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

// MustGetTraceID extracts the trace ID from the fiber context.
// Panics if no trace ID is set (use only when Trace middleware is guaranteed).
func MustGetTraceID(c *fiber.Ctx) string {
	v := GetTraceID(c)
	if v == "" {
		panic("trace ID not found in context - ensure Trace middleware is configured")
	}
	return v
}

// TraceIDFromContext creates a map entry for trace ID suitable for logging.
//
// Usage with zerolog:
//
//	logger.Info().Fields(middleware.TraceIDFromContext(c)).Msg("processing request")
func TraceIDFromContext(c *fiber.Ctx) map[string]interface{} {
	traceID := GetTraceID(c)
	if traceID == "" {
		return nil
	}
	return map[string]interface{}{
		"trace_id": traceID,
	}
}
