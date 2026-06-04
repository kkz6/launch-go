// Package policies declares the database module's authorization abilities and
// registers them into the shared access gate. Abilities are role-based: team
// ownership of a resource is already guaranteed by TeamScope middleware and the
// service layer's FindByIDAndTeam, so each ability maps to the minimum team
// role required to perform the action.
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the database module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// Core database lifecycle.
	g.Define("database.view", member)
	g.Define("database.create", editor)
	g.Define("database.delete", admin)

	// Sync action: re-reads database state from the server. Treated as a
	// write-level action because it enqueues an SSH job.
	g.Define("database.sync", editor)

	// Database user sub-resource.
	g.Define("database.user.view", member)
	g.Define("database.user.create", editor)
	g.Define("database.user.update", editor)
	g.Define("database.user.delete", admin)
}
