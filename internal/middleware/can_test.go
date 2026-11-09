package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// canTestGate builds a gate with one role-based ability.
func canTestGate() *access.Gate {
	g := access.New()
	g.Define("server.delete", authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin)))
	g.Define("site.deploy", authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor)))
	return g
}

// appWithCan wires a route guarded by Can, injecting a team role into context.
func appWithCan(role string) *fiber.App {
	InitGate(canTestGate())
	app := fiber.New()
	app.Use(Locale(i18n.LocaleEnglish))
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(fiberctx.KeyUserID, "u1")
		c.Locals(fiberctx.KeyTeamID, "t1")
		c.Locals(fiberctx.KeyTeamRole, role)
		return c.Next()
	})
	app.Delete("/servers/:id", Can("server.delete"), func(c *fiber.Ctx) error {
		return c.SendString("deleted")
	})
	app.Post("/sites/:id/deploy", Can("site.deploy"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
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

func TestCan_DeployRoleAndLocalizedDenial(t *testing.T) {
	t.Cleanup(func() { InitGate(nil) })
	for _, role := range []string{"owner", "admin", "editor", "member", ""} {
		t.Run(role, func(t *testing.T) {
			app := appWithCan(role)
			request := httptest.NewRequest("POST", "/sites/s1/deploy", nil)
			request.Header.Set("Accept-Language", "ja")
			response, err := app.Test(request)
			require.NoError(t, err)
			defer response.Body.Close()
			if role != "member" && role != "" {
				assert.Equal(t, fiber.StatusOK, response.StatusCode)
				return
			}
			assert.Equal(t, fiber.StatusForbidden, response.StatusCode)
			var body fiberctx.ErrorResponse
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
			assert.Equal(t, "permission.denied", body.Code)
			assert.Equal(t, "現在のチームでの役割では、この操作を実行できません。チームのオーナーに権限の変更を依頼してください。", body.Message)
		})
	}
}
