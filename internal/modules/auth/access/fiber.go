package access

import (
	"github.com/gofiber/fiber/v2"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ActorFromFiber assembles an Actor from the request context. It must run after
// the Auth and TeamScope middleware, which populate the user, team, and role
// locals. Missing values yield zero-value fields (e.g. empty TeamRole), which
// policies treat as "no privilege".
func ActorFromFiber(c *fiber.Ctx) Actor {
	userID, _ := fiberctx.GetUserID(c)
	teamID, _ := fiberctx.GetTeamID(c)
	return Actor{
		UserID:   userID,
		TeamID:   teamID,
		TeamRole: authtypes.TeamRole(fiberctx.GetUserRole(c)),
		ReadOnly: fiberctx.IsImpersonationReadOnly(c),
	}
}
