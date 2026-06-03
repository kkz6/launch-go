package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

func appWithStaff(role *stafftypes.StaffRole, minRole stafftypes.StaffRole) *fiber.App {
	InitStaffMiddleware(func(userID string) *stafftypes.StaffRole { return role })
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("userID", "u1"); return c.Next() })
	app.Get("/admin", RequireStaff(minRole), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func TestRequireStaff_NullRoleForbidden(t *testing.T) {
	app := appWithStaff(nil, stafftypes.StaffRoleSupport)
	resp, err := app.Test(httptest.NewRequest("GET", "/admin", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestRequireStaff_SupportBlockedFromSuperAdmin(t *testing.T) {
	role := stafftypes.StaffRoleSupport
	app := appWithStaff(&role, stafftypes.StaffRoleSuperAdmin)
	resp, _ := app.Test(httptest.NewRequest("GET", "/admin", nil))
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestRequireStaff_SuperAdminAllowed(t *testing.T) {
	role := stafftypes.StaffRoleSuperAdmin
	app := appWithStaff(&role, stafftypes.StaffRoleSupport)
	resp, _ := app.Test(httptest.NewRequest("GET", "/admin", nil))
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
