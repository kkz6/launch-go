package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

const (
	iconGET    = "📥"
	iconPOST   = "📤"
	iconPUT    = "📝"
	iconPATCH  = "🔧"
	iconDELETE = "🗑️ "
	iconOPTION = "⚙️ "
)

func getMethodIcon(method string) string {
	switch method {
	case "GET":
		return iconGET
	case "POST":
		return iconPOST
	case "PUT":
		return iconPUT
	case "PATCH":
		return iconPATCH
	case "DELETE":
		return iconDELETE
	case "OPTIONS":
		return iconOPTION
	default:
		return "📡"
	}
}

func getStatusColor(status int) string {
	switch {
	case status >= 500:
		return colorRed
	case status >= 400:
		return colorYellow
	case status >= 300:
		return colorCyan
	case status >= 200:
		return colorGreen
	default:
		return colorWhite
	}
}

func getStatusIcon(status int) string {
	switch {
	case status >= 500:
		return "💥"
	case status >= 400:
		return "⚠️ "
	case status >= 300:
		return "↪️ "
	case status >= 200:
		return "✅"
	default:
		return "❓"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.0fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func RequestLogger(logger *zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		latency := time.Since(start)
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		ip := c.IP()

		var event *zerolog.Event
		switch {
		case status >= 500:
			event = logger.Error()
		case status >= 400:
			event = logger.Warn()
		default:
			event = logger.Info()
		}

		event.
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Str("latency", formatDuration(latency)).
			Str("ip", ip).
			Msgf("%s %s %s%s%d%s %s",
				getMethodIcon(method),
				method,
				getStatusColor(status),
				getStatusIcon(status),
				status,
				colorReset,
				path,
			)

		return err
	}
}

func RequestLoggerWithBody(logger *zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		reqSize := len(c.Body())

		err := c.Next()

		latency := time.Since(start)
		status := c.Response().StatusCode()
		resSize := len(c.Response().Body())

		var event *zerolog.Event
		switch {
		case status >= 500:
			event = logger.Error()
		case status >= 400:
			event = logger.Warn()
		default:
			event = logger.Info()
		}

		event.
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Str("latency", formatDuration(latency)).
			Str("ip", c.IP()).
			Int("req_size", reqSize).
			Int("res_size", resSize).
			Msgf("%s %s %s%s%d%s %s",
				getMethodIcon(c.Method()),
				c.Method(),
				getStatusColor(status),
				getStatusIcon(status),
				status,
				colorReset,
				c.Path(),
			)

		return err
	}
}
