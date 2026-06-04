package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// canTestGate builds a gate with one role-based ability.
func canTestGate() *access.Gate {
	g := access.New()
	g.Define("server.delete", authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin)))
	return g
}

// appWithCan wires a route guarded by Can, injecting a team role into context.
func appWithCan(role string) *fiber.App {
	InitGate(canTestGate())
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(fiberctx.KeyUserID, "u1")
		c.Locals(fiberctx.KeyTeamID, "t1")
		c.Locals(fiberctx.KeyTeamRole, role)
		return c.Next()
	})
	app.Delete("/servers/:id", Can("server.delete"), func(c *fiber.Ctx) error {
		return c.SendString("deleted")
	})
	return app
}

func TestCan_AllowsSufficientRole(t *testing.T) {
	app := appWithCan("admin")
	resp, err := app.Test(httptest.NewRequest("DELETE", "/servers/s1", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestCan_DeniesInsufficientRole(t *testing.T) {
	app := appWithCan("editor")
	resp, err := app.Test(httptest.NewRequest("DELETE", "/servers/s1", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestCan_OwnerInheritsAdmin(t *testing.T) {
	app := appWithCan("owner")
	resp, err := app.Test(httptest.NewRequest("DELETE", "/servers/s1", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
