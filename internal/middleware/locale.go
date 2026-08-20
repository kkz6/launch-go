package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// Locale detects the request language before handlers run. Auth middleware
// may later replace it with the authenticated user's persisted preference.
func Locale(defaultLocale string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		resolved := i18n.DetectAcceptLanguage(c.Get("Accept-Language"), defaultLocale)
		i18n.SetDetectedLocale(c, resolved)

		// CORS and other middleware may also add Vary values while the request
		// unwinds, so merge ours after downstream handlers have completed.
		defer i18n.AddVary(c, "Accept-Language")
		return c.Next()
	}
}
