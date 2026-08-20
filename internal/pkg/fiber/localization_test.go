package fiber

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/apperror"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

func japaneseTestApp() *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler()})
	app.Use(func(c *fiber.Ctx) error {
		i18n.SetDetectedLocale(c, i18n.LocaleJapanese)
		return c.Next()
	})
	return app
}

func TestSuccessResponseUsesRequestLocale(t *testing.T) {
	app := japaneseTestApp()
	app.Get("/", func(c *fiber.Ctx) error { return OK(c, "User retrieved", nil) })

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	var body ErrorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.NotEqual(t, "User retrieved", body.Message)
	assert.Equal(t, i18n.LocaleJapanese, resp.Header.Get("Content-Language"))
}

func TestAppErrorKeepsCodeAndTranslatesMessage(t *testing.T) {
	app := japaneseTestApp()
	app.Get("/", func(c *fiber.Ctx) error {
		return apperror.ErrConflict.WithCode("auth.email_taken").WithMessage("Resource conflict")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	var body ErrorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "auth.email_taken", body.Code)
	assert.NotEqual(t, "Resource conflict", body.Message)
}

func TestValidationResponseUsesRequestLocale(t *testing.T) {
	type request struct {
		Name string `json:"name" validate:"required"`
	}

	app := japaneseTestApp()
	app.Post("/", Validate(func(c *fiber.Ctx, req *request) error {
		return OK(c, "Created successfully", req)
	}))

	httpReq := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{}`))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	var body ValidationErrorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.NotEqual(t, "Validation failed", body.Message)
	require.NotEmpty(t, body.Errors["name"])
	assert.NotEqual(t, "This field is required", body.Errors["name"][0])
}
