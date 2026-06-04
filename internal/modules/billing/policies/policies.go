// Package policies declares the billing module's authorization abilities and
// registers them into the shared access gate. Abilities are role-based: team
// ownership is guaranteed by TeamScope middleware, so each ability maps to the
// minimum team role required to perform the action.
//
// Default matrix:
//   - read:  member  (view billing info, plans, subscriptions, orders, options)
//   - write: owner   (subscription lifecycle changes — checkout, cancel, resume —
//     are financial/ownership operations and require the owner role)
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the billing module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	owner := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleOwner))

	// Billing inspection (plans, subscriptions, orders, options).
	g.Define("billing.view", member)

	// Subscription lifecycle mutations are financial operations — owner only.
	g.Define("billing.update", owner)
}
