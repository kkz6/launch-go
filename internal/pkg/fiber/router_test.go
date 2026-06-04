package fiber

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouter_AutoWrapsRequestHandler(t *testing.T) {
	var gotTeam string
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(KeyTeamID, "team-1")
		c.Locals(KeyUserID, "user-1")
		return c.Next()
	})
	r := Wrap(app)
	// Pass a func(*Request) error directly — no fiberutil.Handler(...) wrapper.
	r.Get("/x", func(req *Request) error {
		gotTeam = req.TeamID
		return req.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "team-1", gotTeam)
}

func TestRouter_PassesPlainFiberHandler(t *testing.T) {
	app := fiber.New()
	r := Wrap(app)
	// A plain fiber handler is used as-is.
	r.Get("/y", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/y", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

func TestRouter_MiddlewareRunsBeforeHandler(t *testing.T) {
	var order []string
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(KeyTeamID, "t")
		c.Locals(KeyUserID, "u")
		return c.Next()
	})
	r := Wrap(app)
	mw := func(c *fiber.Ctx) error { order = append(order, "mw"); return c.Next() }
	r.Post("/z", func(req *Request) error {
		order = append(order, "handler")
		return req.SendStatus(fiber.StatusOK)
	}, mw)

	resp, err := app.Test(httptest.NewRequest("POST", "/z", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{"mw", "handler"}, order, "middleware must run before the handler")
}

func TestRouter_AcceptsBindResult(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(KeyTeamID, "t")
		c.Locals(KeyUserID, "u")
		return c.Next()
	})
	r := Wrap(app)
	// Body handlers are wrapped explicitly with Bind; the result is a
	// fiber.Handler the router passes through unchanged.
	r.Post("/b", Bind(func(req *Request, body *bindTestReq) error {
		return req.SendStatus(fiber.StatusOK)
	}))

	httpReq := httptest.NewRequest("POST", "/b", nil)
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	// empty body fails validation (Name required) -> not 200, but importantly
	// the route registered and ran without panicking.
	assert.NotEqual(t, fiber.StatusOK, resp.StatusCode)
}
