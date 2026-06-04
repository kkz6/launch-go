// Package policies declares the server module's authorization abilities and
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

// Register wires the server module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// Core server lifecycle.
	g.Define("server.view", member)
	g.Define("server.create", editor)
	g.Define("server.update", editor)
	g.Define("server.delete", admin)

	// Server management actions (provisioned servers).
	g.Define("server.manage", editor) // crons, daemons, firewall, ssh keys, php, etc.
	g.Define("server.reboot", admin)
	g.Define("server.audit", member)

	// Server provider connections.
	g.Define("server_provider.view", member)
	g.Define("server_provider.create", admin)
	g.Define("server_provider.delete", admin)
}
