// Package policies declares the certificate module's authorization abilities
// and registers them into the shared access gate. Abilities are role-based:
// team ownership of a resource is already guaranteed by TeamScope middleware
// and the service layer's FindByIDAndTeam, so each ability maps to the
// minimum team role required to perform the action.
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the certificate module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// Core certificate lifecycle.
	g.Define("certificate.view", member)
	g.Define("certificate.create", editor)
	g.Define("certificate.update", editor)
	g.Define("certificate.delete", admin)

	// Usages is a read-only inspection endpoint (no separate ability; covered
	// by certificate.view enforced on the GET handler).
}
