package middleware

import (
	"time"

	"github.com/getsentry/sentry-go"
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/config"
)

// InitSentry initializes Sentry error tracking
// Returns true if Sentry was successfully initialized, false otherwise
func InitSentry(cfg config.SentryConfig, appName string, release string, logger *zerolog.Logger) bool {
	if !cfg.IsEnabled() {
		logger.Info().Msg("Sentry is disabled (not in production or DSN not set)")
		return false
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Debug:            cfg.Debug,
		SampleRate:       cfg.SampleRate,
		EnableTracing:    cfg.TracesSampleRate > 0,
		TracesSampleRate: cfg.TracesSampleRate,
		Release:          release,
		ServerName:       appName,
		BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			return event
		},
	})

	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize Sentry")
		return false
	}

	logger.Info().
		Str("environment", cfg.Environment).
		Str("dsn", maskDSN(cfg.DSN)).
		Msg("Sentry initialized successfully")

	return true
}

// FlushSentry flushes any buffered events to Sentry
// Call this during graceful shutdown
func FlushSentry(timeout time.Duration) {
	sentry.Flush(timeout)
}

// SentryHandler returns the Sentry middleware for Fiber
func SentryHandler() fiber.Handler {
	return sentryfiber.New(sentryfiber.Options{
		Repanic:         true,  // Re-panic after capturing so Fiber's recover middleware can handle it
		WaitForDelivery: false, // Don't block request for Sentry delivery (async)
	})
}

// SentryHandlerWithOptions returns the Sentry middleware with custom options
func SentryHandlerWithOptions(opts sentryfiber.Options) fiber.Handler {
	return sentryfiber.New(opts)
}

// EnhanceSentryScope is a middleware that adds custom tags/context to Sentry events
func EnhanceSentryScope(c *fiber.Ctx) error {
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.Scope().SetTag("request_id", c.GetRespHeader("X-Request-ID"))

		// Add user context if available
		if userID, ok := c.Locals("userID").(string); ok && userID != "" {
			hub.Scope().SetUser(sentry.User{
				ID: userID,
			})
		}

		// Add team context if available
		if teamID, ok := c.Locals("teamID").(string); ok && teamID != "" {
			hub.Scope().SetTag("team_id", teamID)
		}
	}
	return c.Next()
}

// CaptureError captures an error to Sentry with the current context
func CaptureError(c *fiber.Ctx, err error) {
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.CaptureException(err)
	}
}

// CaptureMessage captures a message to Sentry with the current context
func CaptureMessage(c *fiber.Ctx, message string) {
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.CaptureMessage(message)
	}
}

// AddBreadcrumb adds a breadcrumb to the current Sentry scope
func AddBreadcrumb(c *fiber.Ctx, breadcrumb *sentry.Breadcrumb) {
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.AddBreadcrumb(breadcrumb, nil)
	}
}

// SetTag sets a tag on the current Sentry scope
func SetTag(c *fiber.Ctx, key, value string) {
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.Scope().SetTag(key, value)
	}
}

// SetExtra sets extra data on the current Sentry scope
func SetExtra(c *fiber.Ctx, key string, value interface{}) {
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.Scope().SetExtra(key, value)
	}
}

// maskDSN masks the DSN for logging (shows only the host)
func maskDSN(dsn string) string {
	if len(dsn) < 20 {
		return "***"
	}
	// Show only the first 20 characters
	return dsn[:20] + "..."
}
