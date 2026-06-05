package middleware

import (
	"github.com/gofiber/fiber/v2"

	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	"github.com/kkz6/launch-go/internal/pkg/access"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

var gateMiddleware struct {
	gate *access.Gate
}

// InitGate wires the authorization gate used by Can. Call during bootstrap
// after the gate is built and all module policies are registered.
func InitGate(gate *access.Gate) {
	gateMiddleware.gate = gate
}

// Can guards a route with an authorization ability (e.g. "server.delete"). It
// builds the Actor from request context and asks the gate to authorize the
// ability. It must run after Auth and TeamScope so the user/team/role context
// is populated. Resource-level team ownership is already enforced by TeamScope
// and the service layer's FindByIDAndTeam, so the gate evaluates the per-action
// role policy with no resource instance.
//
// If no gate is wired (tests or contexts that don't need authorization), Can is
// a no-op pass-through.
func Can(ability string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if gateMiddleware.gate == nil {
			return c.Next()
		}

		actor := authaccess.ActorFromFiber(c)
		if err := gateMiddleware.gate.Authorize(c.Context(), actor, ability, nil); err != nil {
			return fiberctx.RespondForbidden(c, err.Error())
		}

		return c.Next()
	}
}
