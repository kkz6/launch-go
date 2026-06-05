// Package policies declares the auth module's own authorization abilities
// (teams, members, sessions, tokens) and registers them into the shared access
// gate. Abilities are role-based; team membership is enforced upstream.
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the auth module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))
	owner := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleOwner))

	// Team settings — only the owner manages the team itself.
	g.Define("team.view", member)
	g.Define("team.update", owner)
	g.Define("team.delete", owner)

	// Members — owner/admin manage membership.
	g.Define("team.member.view", member)
	g.Define("team.member.invite", admin)
	g.Define("team.member.update", owner)
	g.Define("team.member.remove", admin)
	g.Define("team.invitation.view", admin)
}
