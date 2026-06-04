// Package policies declares the platform module's authorization abilities
// and registers them into the shared access gate. Abilities are role-based:
// team ownership is guaranteed by TeamScope middleware, so each ability maps
// to the minimum team role required to perform the action.
//
// Default matrix:
//   - read:   member  (list and view pending updates)
//   - write:  editor  (run an update, run-all, dismiss a banner)
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the platform module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))

	// Platform update inspection.
	g.Define("platform.view", member)

	// Triggering and dismissing updates modifies server state.
	g.Define("platform.update", editor)
}
