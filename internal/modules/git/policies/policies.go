// Package policies declares the git module's authorization abilities and
// registers them into the shared access gate. Abilities are role-based: team
// ownership of a resource is already guaranteed by TeamScope middleware and the
// service layer's FindByIDAndTeam, so each ability maps to the minimum team
// role required to perform the action.
//
// OAuth callbacks require the current session and a team administrator.
// Public webhook routes instead verify the provider signature.
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the git module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// Source-control connection views (read-only, any member may list/inspect).
	g.Define("git.source_control.view", member)

	// Connecting and reviewing repository grants requires a team administrator.
	g.Define("git.source_control.connect", admin)

	// Refreshing / syncing cached repositories is a write operation; editors
	// may trigger background sync jobs.
	g.Define("git.source_control.sync", editor)

	// Disconnecting archives authority and revokes credentials; history remains.
	g.Define("git.source_control.delete", admin)
}
