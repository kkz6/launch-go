// Package policies declares the docker module's authorization abilities and
// registers them into the shared access gate. Abilities are role-based: team
// ownership of a resource is already guaranteed by TeamScope middleware and the
// service layer's FindByIDAndTeam, so each ability maps to the minimum team
// role required to perform the action.
//
// Default matrix:
//   - read (view/list/show/logs/deployments/services/inspect): member
//   - write (create/update/deploy/start/stop/restart/reload/redeploy/
//     add-domain/update-domain/rotate-token/resync/auto-deploy/expose/
//     env-var writes/build-secret writes/volume writes/schedule writes/
//     backup configure): editor
//   - delete of a top-level resource (project/application/compose/database)
//     and destructive data actions (rebuild/restore/backup-delete/purge):
//     admin
//
// Registry credentials are team-scoped, not server-scoped, so they follow the
// same matrix (member=view, editor=write, admin=delete).
package policies

import (
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Register wires the docker module's abilities into the gate.
func Register(g *access.Gate) {
	member := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleMember))
	editor := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleEditor))
	admin := authaccess.Policy(authaccess.RequireRole(authtypes.TeamRoleAdmin))

	// -------------------------------------------------------------------------
	// Projects
	// -------------------------------------------------------------------------
	g.Define("docker.project.view", member)
	g.Define("docker.project.create", editor)
	g.Define("docker.project.update", editor)
	g.Define("docker.project.delete", admin)

	// Project-level env vars (source for ${{project.<KEY>}} references).
	g.Define("docker.project.env_var.view", member)
	g.Define("docker.project.env_var.create", editor)
	g.Define("docker.project.env_var.update", editor)
	g.Define("docker.project.env_var.delete", editor)

	// -------------------------------------------------------------------------
	// Applications
	// -------------------------------------------------------------------------
	g.Define("docker.application.view", member)
	g.Define("docker.application.create", editor)
	g.Define("docker.application.update", editor)
	g.Define("docker.application.delete", admin)

	// Deployment / lifecycle actions.
	g.Define("docker.application.deploy", editor)
	g.Define("docker.application.start", editor)
	g.Define("docker.application.stop", editor)
	g.Define("docker.application.restart", editor)

	// GitHub Actions management.
	g.Define("docker.application.gha.rotate_token", editor)
	g.Define("docker.application.gha.resync", editor)
	g.Define("docker.application.gha.disable", editor)
	g.Define("docker.application.gha.auto_deploy", editor)

	// Domains.
	g.Define("docker.application.domain.view", member)
	g.Define("docker.application.domain.add", editor)
	g.Define("docker.application.domain.update", editor)
	g.Define("docker.application.domain.remove", editor)

	// Redirects.
	g.Define("docker.application.redirect.view", member)
	g.Define("docker.application.redirect.create", editor)
	g.Define("docker.application.redirect.update", editor)
	g.Define("docker.application.redirect.delete", editor)

	// Env vars (runtime).
	g.Define("docker.application.env_var.view", member)
	g.Define("docker.application.env_var.create", editor)
	g.Define("docker.application.env_var.update", editor)
	g.Define("docker.application.env_var.set", editor) // bulk PUT
	g.Define("docker.application.env_var.delete", editor)

	// Build secrets (write-only; value never returned).
	g.Define("docker.application.build_secret.view", member)
	g.Define("docker.application.build_secret.create", editor)
	g.Define("docker.application.build_secret.update", editor)
	g.Define("docker.application.build_secret.delete", editor)

	// Volumes.
	g.Define("docker.application.volume.view", member)
	g.Define("docker.application.volume.create", editor)
	g.Define("docker.application.volume.update", editor)
	g.Define("docker.application.volume.delete", editor)

	// Schedules.
	g.Define("docker.application.schedule.view", member)
	g.Define("docker.application.schedule.create", editor)
	g.Define("docker.application.schedule.update", editor)
	g.Define("docker.application.schedule.delete", editor)

	// Traefik config (advanced operator action — editor so the same person
	// who deploys can also tune routing without needing admin).
	g.Define("docker.application.traefik_config.view", member)
	g.Define("docker.application.traefik_config.update", editor)

	// Advanced runtime settings (resource limits, restart policy, etc.).
	g.Define("docker.application.advanced.update", editor)

	// -------------------------------------------------------------------------
	// Compose stacks
	// -------------------------------------------------------------------------
	g.Define("docker.compose.view", member)
	g.Define("docker.compose.create", editor)
	g.Define("docker.compose.update", editor)
	g.Define("docker.compose.delete", admin)

	// Deployment / lifecycle.
	g.Define("docker.compose.deploy", editor)
	g.Define("docker.compose.reload", editor)

	// GitHub Actions management (mirrors application GHA abilities).
	g.Define("docker.compose.gha.rotate_token", editor)
	g.Define("docker.compose.gha.resync", editor)
	g.Define("docker.compose.gha.disable", editor)
	g.Define("docker.compose.gha.auto_deploy", editor)

	// Build secrets.
	g.Define("docker.compose.build_secret.view", member)
	g.Define("docker.compose.build_secret.create", editor)
	g.Define("docker.compose.build_secret.update", editor)
	g.Define("docker.compose.build_secret.delete", editor)

	// Volumes.
	g.Define("docker.compose.volume.view", member)
	g.Define("docker.compose.volume.create", editor)
	g.Define("docker.compose.volume.update", editor)
	g.Define("docker.compose.volume.delete", editor)

	// Domains.
	g.Define("docker.compose.domain.view", member)
	g.Define("docker.compose.domain.add", editor)
	g.Define("docker.compose.domain.update", editor)
	g.Define("docker.compose.domain.remove", editor)

	// Traefik config.
	g.Define("docker.compose.traefik_config.view", member)
	g.Define("docker.compose.traefik_config.update", editor)

	// Purge orphaned compose resources (admin: destructive).
	g.Define("docker.compose.purge", admin)

	// -------------------------------------------------------------------------
	// Databases
	// -------------------------------------------------------------------------
	g.Define("docker.database.view", member)
	g.Define("docker.database.create", editor)
	g.Define("docker.database.delete", admin)

	// Lifecycle actions (start/stop/restart).
	g.Define("docker.database.lifecycle", editor)

	// Expose external port.
	g.Define("docker.database.expose", editor)

	// Rebuild (Danger Zone — wipes data volume, admin only).
	g.Define("docker.database.rebuild", admin)

	// Advanced settings.
	g.Define("docker.database.advanced.update", editor)

	// Database env vars.
	g.Define("docker.database.env_var.view", member)
	g.Define("docker.database.env_var.create", editor)
	g.Define("docker.database.env_var.update", editor)
	g.Define("docker.database.env_var.delete", editor)

	// Backup config (configure + delete + run + restore).
	g.Define("docker.database.backup.view", member)
	g.Define("docker.database.backup.configure", editor)
	g.Define("docker.database.backup.delete", admin)
	g.Define("docker.database.backup.run", editor)
	g.Define("docker.database.backup.restore", admin) // restores overwrite data

	// -------------------------------------------------------------------------
	// Host diagnostics (containers/traefik views + traefik file edits)
	// -------------------------------------------------------------------------
	g.Define("docker.host.view", member)
	g.Define("docker.host.traefik.update", editor)

	// -------------------------------------------------------------------------
	// Registry credentials (team-scoped)
	// -------------------------------------------------------------------------
	g.Define("docker.registry_credential.view", member)
	g.Define("docker.registry_credential.create", editor)
	g.Define("docker.registry_credential.update", editor)
	g.Define("docker.registry_credential.delete", admin)
}
