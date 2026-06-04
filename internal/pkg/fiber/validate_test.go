package fiber

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validatedTestReq struct {
	Name string `json:"name" validate:"required"`
}

// TestValidate_InjectsValidatedBody: a valid body is parsed, validated, and
// handed to the handler, which never writes parse/validate boilerplate.
func TestValidate_InjectsValidatedBody(t *testing.T) {
	var got string
	app := fiber.New()
	app.Post("/x", Validate(func(c *fiber.Ctx, req *validatedTestReq) error {
		got = req.Name
		return c.SendStatus(fiber.StatusOK)
	}))

	httpReq := httptest.NewRequest("POST", "/x", strings.NewReader(`{"name":"alice"}`))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "alice", got)
}

// TestValidate_RejectsInvalidBody: a body failing validation must NOT reach the
// handler, and the request must not succeed.
func TestValidate_RejectsInvalidBody(t *testing.T) {
	called := false
	app := fiber.New()
	app.Post("/x", Validate(func(c *fiber.Ctx, req *validatedTestReq) error {
		called = true
		return c.SendStatus(fiber.StatusOK)
	}))

	httpReq := httptest.NewRequest("POST", "/x", strings.NewReader(`{}`))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	assert.False(t, called, "handler not called when validation fails")
	assert.NotEqual(t, fiber.StatusOK, resp.StatusCode)
}
