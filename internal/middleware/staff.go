package middleware

import (
	"github.com/gofiber/fiber/v2"

	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// StaffRoleLoader resolves a user's staff role (nil = not staff).
type StaffRoleLoader func(userID string) *stafftypes.StaffRole

// staffMiddleware holds the shared state for staff middleware.
var staffMiddleware struct {
	load StaffRoleLoader
}

// InitStaffMiddleware wires the staff-role lookup. Call during bootstrap
// before routes are registered.
func InitStaffMiddleware(load StaffRoleLoader) {
	staffMiddleware.load = load
}

// RequireStaff ensures the authenticated user has at least minRole of staff
// access. Must run after the auth middleware (needs c.Locals("userID")).
//
// Usage:
//
//	router.Get("/admin", middleware.RequireStaff(stafftypes.StaffRoleSupport), handler.Index)
func RequireStaff(minRole stafftypes.StaffRole) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if staffMiddleware.load == nil {
			return fiberctx.RespondForbidden(c, "Staff access not configured")
		}

		userID, ok := c.Locals(fiberctx.KeyUserID).(string)
		if !ok || userID == "" {
			return fiberctx.RespondUnauthorized(c, "Authentication required")
		}

		role := staffMiddleware.load(userID)
		if role == nil || role.Level() < minRole.Level() {
			return fiberctx.RespondForbidden(c, "Staff access required")
		}

		return c.Next()
	}
}
