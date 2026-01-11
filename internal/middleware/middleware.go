package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

// HTTP method colors and icons
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorGray   = "\033[90m"
)

// Icons for HTTP methods
const (
	iconGET    = "📥"
	iconPOST   = "📤"
	iconPUT    = "📝"
	iconPATCH  = "🔧"
	iconDELETE = "🗑️ "
	iconOPTION = "⚙️ "
)

// getMethodIcon returns an icon for HTTP method
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

// getStatusColor returns a color based on status code
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

// getStatusIcon returns an icon based on status code
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

// formatDuration formats duration for display
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.0fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// RequestLogger creates a beautiful request logging middleware
func RequestLogger(logger *zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Calculate latency
		latency := time.Since(start)
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()

		// Get request details
		ip := c.IP()
		userAgent := c.Get("User-Agent")

		// Shorten user agent for display
		if len(userAgent) > 50 {
			userAgent = userAgent[:50] + "..."
		}

		// Build log event based on status
		var event *zerolog.Event
		switch {
		case status >= 500:
			event = logger.Error()
		case status >= 400:
			event = logger.Warn()
		default:
			event = logger.Info()
		}

		// Log with beautiful formatting
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

// RequestLoggerWithBody creates a request logger that also logs request/response body sizes
func RequestLoggerWithBody(logger *zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		reqSize := len(c.Body())

		// Process request
		err := c.Next()

		// Calculate metrics
		latency := time.Since(start)
		status := c.Response().StatusCode()
		resSize := len(c.Response().Body())

		// Build log event
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

func Auth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Missing authorization header")
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return response.Unauthorized(c, "Invalid authorization format")
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return response.Unauthorized(c, "Invalid token")
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return response.Unauthorized(c, "Invalid token claims")
		}

		// Set user info in context
		c.Locals("userID", claims["sub"])
		c.Locals("teamID", claims["team_id"])

		return c.Next()
	}
}

func TeamScope() fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamID := c.Locals("teamID")
		if teamID == nil || teamID == "" {
			return response.Forbidden(c, "Team context required")
		}
		return c.Next()
	}
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"message": err.Error(),
	})
}
