// Package policies declares the site module's authorization abilities and
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

// Register wires the site module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// Suppress unused-variable warning for member; it is used on site.view.
	_ = member

	// Core site lifecycle.
	g.Define("site.view", member)
	g.Define("site.create", editor)
	g.Define("site.update", editor)
	g.Define("site.delete", admin)

	// Deploy-token regeneration is admin-level: the token grants unauthenticated
	// deploy access to the site and rotating it invalidates all existing webhook
	// integrations. Editors can deploy but should not be able to rotate shared
	// credentials.
	g.Define("site.deploy_token.regenerate", admin)

	// Deployments: triggering, rolling back, and cancelling are editor-level
	// write actions. Auto-deployment toggle is treated the same — it's a
	// persistent write to the site's deployment settings.
	g.Define("site.deployment.create", editor)
	g.Define("site.deployment.rollback", editor)
	g.Define("site.deployment.cancel", editor)
	g.Define("site.deployment.autodeploy", editor)

	// SSL: updating SSL settings provisions or deprovisions a certificate which
	// is a write action. Listing certificates is read-only (no guard needed).
	g.Define("site.ssl.update", editor)

	// Queues: CRUD and operational actions.
	// Delete is admin-level (removes a supervised daemon from the server).
	g.Define("site.queue.create", editor)
	g.Define("site.queue.update", editor) // covers PATCH :queueId and PUT auto-restart-queue
	g.Define("site.queue.sync", editor)
	g.Define("site.queue.restart", editor)
	g.Define("site.queue.delete", admin)

	// Commands: creating and running a command on the server is editor-level.
	// Deleting a saved command is admin-level (permanent removal).
	g.Define("site.command.create", editor)
	g.Define("site.command.delete", admin)

	// Redirects: CRUD for URL redirect rules.
	g.Define("site.redirect.create", editor)
	g.Define("site.redirect.update", editor)
	g.Define("site.redirect.delete", admin)

	// Files: editing config files on the server (e.g. .env, Caddyfile).
	// Listing and reading are GET routes — no guard needed.
	g.Define("site.file.update", editor)

	// Features: enabling/disabling Laravel features (Octane, Horizon, Queue,
	// Scheduler, Reverb, Inertia) triggers server-side provisioning tasks.
	g.Define("site.feature.enable", editor)
	g.Define("site.feature.disable", editor)
}
