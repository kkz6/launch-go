package middleware

import (
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// RecoverConfig holds configuration for the recover middleware
type RecoverConfig struct {
	// Logger for logging panic details
	Logger *zerolog.Logger

	// EnableStackTrace controls whether stack traces are logged
	EnableStackTrace bool

	// StackTraceHandler is called with the stack trace when a panic occurs
	StackTraceHandler func(c *fiber.Ctx, err interface{}, stack []byte)
}

// DefaultRecoverConfig returns the default recover configuration
func DefaultRecoverConfig() RecoverConfig {
	return RecoverConfig{
		EnableStackTrace: true,
	}
}

// Recover returns a middleware that recovers from panics, logs them,
// and returns a proper error response.
//
// This middleware should be registered early in the middleware chain
// to catch panics from all handlers.
//
// Usage:
//
//	app.Use(middleware.Recover(middleware.RecoverConfig{
//	    Logger: logger,
//	    EnableStackTrace: true,
//	}))
func Recover(config ...RecoverConfig) fiber.Handler {
	cfg := DefaultRecoverConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				// Get stack trace
				stack := debug.Stack()

				// Log the panic
				if cfg.Logger != nil {
					event := cfg.Logger.Error().
						Str("method", c.Method()).
						Str("path", c.Path()).
						Str("ip", c.IP()).
						Interface("panic", r)

					if cfg.EnableStackTrace {
						event = event.Bytes("stack", stack)
					}

					event.Msg("Panic recovered")
				}

				// Call custom stack trace handler if provided
				if cfg.StackTraceHandler != nil {
					cfg.StackTraceHandler(c, r, stack)
				}

				// Return internal server error
				err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": "Internal server error",
				})
			}
		}()

		return c.Next()
	}
}

// RecoverWithLogger is a convenience function that creates a recover middleware with just a logger
func RecoverWithLogger(logger *zerolog.Logger) fiber.Handler {
	return Recover(RecoverConfig{
		Logger:           logger,
		EnableStackTrace: true,
	})
}
