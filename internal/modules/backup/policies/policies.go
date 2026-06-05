// Package policies declares the backup module's authorization abilities and
// registers them into the shared access gate. Abilities are role-based: team
// ownership of a resource is already guaranteed by TeamScope middleware and the
// service layer, so each ability maps to the minimum team role required.
//
// Default matrix:
//   - read:    member  (view backups, backup jobs, storage providers)
//   - write:   editor  (create/update backups, run backup jobs, connect/update storage providers)
//   - run:     editor  (trigger an immediate backup execution)
//   - restore: admin   (a restore is destructive — it overwrites live data)
//   - delete:  admin   (delete top-level resources: backups, storage providers)
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the backup module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// Backup CRUD.
	g.Define("backup.view", member)
	g.Define("backup.create", editor)
	g.Define("backup.update", editor)
	g.Define("backup.delete", admin)

	// Backup execution actions.
	g.Define("backup.run", editor)
	// Restore is destructive (overwrites live data) — require admin.
	g.Define("backup.restore", admin)

	// Storage provider management.
	g.Define("storage_provider.view", member)
	g.Define("storage_provider.create", editor)
	g.Define("storage_provider.update", editor)
	g.Define("storage_provider.delete", admin)
}
