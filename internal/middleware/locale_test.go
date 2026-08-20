package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

func TestLocaleDetectsAndDoesNotLeakBetweenRequests(t *testing.T) {
	app := fiber.New()
	app.Use(Locale(i18n.LocaleEnglish))
	app.Get("/locale", func(c *fiber.Ctx) error {
		c.Set("Vary", "Origin")
		return c.SendString(i18n.Locale(c))
	})

	jaReq := httptest.NewRequest("GET", "/locale", nil)
	jaReq.Header.Set("Accept-Language", "ja-JP, en;q=0.5")
	jaResp, err := app.Test(jaReq)
	require.NoError(t, err)
	assert.Equal(t, i18n.LocaleJapanese, jaResp.Header.Get("Content-Language"))
	assert.Contains(t, strings.Split(jaResp.Header.Get("Vary"), ", "), "Origin")
	assert.Contains(t, strings.Split(jaResp.Header.Get("Vary"), ", "), "Accept-Language")

	enResp, err := app.Test(httptest.NewRequest("GET", "/locale", nil))
	require.NoError(t, err)
	assert.Equal(t, i18n.LocaleEnglish, enResp.Header.Get("Content-Language"))
}

func TestLocalePreferenceOverridesDetection(t *testing.T) {
	app := fiber.New()
	app.Use(Locale(i18n.LocaleEnglish))
	app.Use(func(c *fiber.Ctx) error {
		preferred := i18n.LocaleJapanese
		i18n.ApplyPreference(c, &preferred)
		return c.Next()
	})
	app.Get("/locale", func(c *fiber.Ctx) error { return c.SendString(i18n.Locale(c)) })

	req := httptest.NewRequest("GET", "/locale", nil)
	req.Header.Set("Accept-Language", "en-US")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, i18n.LocaleJapanese, resp.Header.Get("Content-Language"))
}
