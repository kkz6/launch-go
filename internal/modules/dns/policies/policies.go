// Package policies declares the dns module's authorization abilities and
// registers them into the shared access gate. Abilities are role-based: team
// ownership of a resource is already guaranteed by TeamScope middleware and the
// service layer's scoped lookups, so each ability maps to the minimum team
// role required to perform the action.
//
// Default role matrix:
//   - read:   member  (GET endpoints are not guarded — all authenticated
//     team members may read DNS data)
//   - write:  editor  (create / update on providers, domains, records)
//   - delete: admin   (delete providers, domains, records)
//   - owner-only actions (disconnect/sync provider): owner
//     NOTE: provider connectivity check and domain sync are operational
//     actions that can reveal or change provider credentials, so they are
//     restricted to owner level.
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the dns module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))
	owner := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleOwner))

	// DNS provider connections.
	g.Define("dns_provider.view", member)
	g.Define("dns_provider.create", editor)
	g.Define("dns_provider.delete", admin)
	// check and sync touch provider credentials — owner only.
	g.Define("dns_provider.check", owner)
	g.Define("dns_provider.sync", owner)

	// Domain management.
	g.Define("dns.domain.view", member)
	g.Define("dns.domain.create", editor)
	g.Define("dns.domain.update", editor)
	g.Define("dns.domain.delete", admin)
	g.Define("dns.domain.sync", editor)

	// DNS record management (nested under domain).
	g.Define("dns.record.view", member)
	g.Define("dns.record.create", editor)
	g.Define("dns.record.update", editor)
	g.Define("dns.record.delete", editor)
}
