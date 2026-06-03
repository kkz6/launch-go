package middleware

import (
	"github.com/gofiber/fiber/v2"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// suspendedAccountMessage is the response shown when a suspended user is blocked.
const suspendedAccountMessage = "Account suspended"

// UserStatusLoader resolves a user's account status.
type UserStatusLoader func(userID string) authtypes.UserStatus

var userStatusMiddleware struct {
	load UserStatusLoader
}

// InitUserStatus wires the account-status lookup. Call during bootstrap.
func InitUserStatus(load UserStatusLoader) {
	userStatusMiddleware.load = load
}

// isUserSuspended reports whether the authenticated user is suspended.
// Safe default: if no loader or empty id, NOT suspended (fail open is acceptable
// here because the loader is always wired in production; missing user => active).
func isUserSuspended(c *fiber.Ctx) bool {
	if userStatusMiddleware.load == nil {
		return false
	}

	userID, ok := c.Locals(fiberctx.KeyUserID).(string)
	if !ok || userID == "" {
		return false
	}

	return userStatusMiddleware.load(userID) == authtypes.UserStatusSuspended
}
