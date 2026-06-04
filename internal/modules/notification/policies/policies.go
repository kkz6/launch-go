// Package policies declares the notification module's authorization abilities
// and registers them into the shared access gate. Abilities are role-based:
// team ownership is guaranteed by TeamScope middleware, so each ability maps
// to the minimum team role required to perform the action.
//
// Default matrix:
//   - read:   member  (list channels, view channel, get preferences, list channel types)
//   - write:  editor  (create channel, update channel, update preferences, set default,
//     test, connect/disconnect/reconnect)
//   - delete: admin   (delete channel)
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the notification module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// Notification channel CRUD.
	g.Define("notification.view", member)
	g.Define("notification.create", editor)
	g.Define("notification.update", editor)
	g.Define("notification.delete", admin)

	// Channel lifecycle actions (test, set-default, disconnect, reconnect).
	g.Define("notification.manage", editor)
}
