// Package policies declares the script module's authorization abilities and
// registers them into the shared access gate. Abilities are role-based: team
// ownership of a resource is already guaranteed by TeamScope middleware and the
// service layer, so each ability maps to the minimum team role required to
// perform the action.
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the script module's abilities into the gate.
//
// Role matrix:
//   - script.view         member  — listing and reading scripts / executions is safe for all members
//   - script.create       editor  — authoring a new script template
//   - script.update       editor  — modifying an existing script template
//   - script.delete       admin   — removing a script template is destructive (team-wide effect)
//   - script.execute      editor  — running a script on one or more servers is potentially destructive;
//     editor (not member) is required so read-only members cannot trigger
//     arbitrary remote commands. Admin is not required because script
//     execution is a day-to-day deployment task that editors own.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// Core script CRUD.
	g.Define("script.view", member)
	g.Define("script.create", editor)
	g.Define("script.update", editor)
	g.Define("script.delete", admin)

	// Script execution — potentially destructive remote command; requires editor.
	g.Define("script.execute", editor)
}
