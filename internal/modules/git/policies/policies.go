// Package policies declares the git module's authorization abilities and
// registers them into the shared access gate. Abilities are role-based: team
// ownership of a resource is already guaranteed by TeamScope middleware and the
// service layer's FindByIDAndTeam, so each ability maps to the minimum team
// role required to perform the action.
//
// OAuth callbacks and public webhook routes are NOT governed by the gate —
// they run outside TeamScope and carry no team-role context.
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

	// Connecting a new git provider app installation requires editor or above.
	// This covers POST /source-controls and the OAuth callback that finalises
	// the install (HandleInstallationCallback performs SyncUserInstallation).
	g.Define("git.source_control.connect", editor)

	// Refreshing / syncing cached repositories is a write operation; editors
	// may trigger background sync jobs.
	g.Define("git.source_control.sync", editor)

	// Disconnecting a source-control connection is destructive (removes the
	// installation record and all cached repositories); restricted to admins.
	g.Define("git.source_control.delete", admin)
}
