package middleware

import (
	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// readOnlyImpersonationMessage is the forbidden response body returned when a
// mutating request is made under a read-only impersonation session. It is the
// single source of truth for this message, shared by Auth() (the central
// enforcement point) and BlockImpersonationWrites().
const readOnlyImpersonationMessage = "Read-only impersonation session"

// BlockImpersonationWrites rejects state-changing requests made under a
// read-only impersonation ("spectate") session. Staff impersonate customers
// in read-only mode; any mutating method while impersonating is denied. It is
// a no-op for normal (non-impersonation) requests.
//
// The read_only flag is sourced from the minted impersonation token's claims
// (surfaced into locals by setAuthContext), so this must run AFTER the auth
// middleware. Because the staff exit flow (POST /admin/impersonate/stop) is
// called with the STAFF token — which carries no read_only claim — this block
// never interferes with stopping an impersonation session.
//
// NOTE: This is the documented, unit-tested policy primitive for the read-only
// contract, but it is NOT wired into the production middleware chains. The
// actual enforcement lives in Auth() (see auth.go), which calls the shared
// isImpersonationWriteBlocked helper so the block applies to every
// authenticated route — including those wired with a bare authMiddleware inline
// that never pass through the chain helpers.
func BlockImpersonationWrites() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if isImpersonationWriteBlocked(c) {
			return fiberctx.RespondForbidden(c, readOnlyImpersonationMessage)
		}

		return c.Next()
	}
}

func isImpersonationWriteBlocked(c *fiber.Ctx) bool {
	ro, _ := c.Locals("impersonationReadOnly").(bool)
	if !ro {
		return false
	}

	switch c.Method() {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		return true
	default:
		return false
	}
}
