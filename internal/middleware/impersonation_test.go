package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// appWithImpersonation builds a Fiber app whose route is guarded by
// BlockImpersonationWrites. When readOnly is true an inline middleware sets the
// impersonationReadOnly local before the block runs, mirroring what
// setAuthContext does for a real impersonation token.
func appWithImpersonation(readOnly bool) *fiber.App {
	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		if readOnly {
			c.Locals("impersonationReadOnly", true)
		}

		return c.Next()
	})

	handler := func(c *fiber.Ctx) error {
		return c.SendString("ok")
	}

	app.Get("/resource", BlockImpersonationWrites(), handler)
	app.Post("/resource", BlockImpersonationWrites(), handler)
	app.Put("/resource", BlockImpersonationWrites(), handler)
	app.Patch("/resource", BlockImpersonationWrites(), handler)
	app.Delete("/resource", BlockImpersonationWrites(), handler)

	return app
}

func TestBlockImpersonationWrites_ReadOnlyBlocksMutations(t *testing.T) {
	app := appWithImpersonation(true)

	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		resp, err := app.Test(httptest.NewRequest(method, "/resource", nil))
		require.NoError(t, err)
		assert.Equalf(t, fiber.StatusForbidden, resp.StatusCode, "%s should be blocked", method)
	}
}

func TestBlockImpersonationWrites_ReadOnlyAllowsReads(t *testing.T) {
	app := appWithImpersonation(true)

	resp, err := app.Test(httptest.NewRequest("GET", "/resource", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestBlockImpersonationWrites_NormalRequestAllowsWrites(t *testing.T) {
	app := appWithImpersonation(false)

	resp, err := app.Test(httptest.NewRequest("POST", "/resource", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
