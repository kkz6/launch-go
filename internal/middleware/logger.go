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

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.0fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func RequestLogger(logger *zerolog.Logger, isDev bool) fiber.Handler {
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
			Str("ip", ip)

		if isDev {
			event.Msgf("%s %s%d%s %s", method, getStatusColor(status), status, colorReset, path)
		} else {
			event.Msgf("%s %d %s", method, status, path)
		}

		return err
	}
}

func RequestLoggerWithBody(logger *zerolog.Logger, isDev bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		reqSize := len(c.Body())

		err := c.Next()

		latency := time.Since(start)
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
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
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Str("latency", formatDuration(latency)).
			Str("ip", c.IP()).
			Int("req_size", reqSize).
			Int("res_size", resSize)

		if isDev {
			event.Msgf("%s %s%d%s %s", method, getStatusColor(status), status, colorReset, path)
		} else {
			event.Msgf("%s %d %s", method, status, path)
		}

		return err
	}
}
