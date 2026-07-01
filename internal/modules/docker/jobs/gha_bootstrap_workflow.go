package jobs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	gitcontracts "github.com/kkz6/launch-go/internal/modules/git/contracts"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"

	"gorm.io/gorm"

	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	dockernotifications "github.com/kkz6/launch-go/internal/modules/docker/notifications"
	dockertasks "github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeGHABootstrapWorkflow is the asynq task type for the
// "commit/refresh GitHub Actions workflow + secrets/variables" job.
// One-shot per workload-event; idempotent on re-run (no-op if the
// workflow YAML on the repo matches what we'd render now).
const TypeGHABootstrapWorkflow = "docker:gha_bootstrap_workflow"

// GHABootstrapWorkflowPayload selects which workload to bootstrap and
// whether to rotate the per-workload deploy token. Bootstrap is the
// same job for create + re-sync + rotate paths; the RotateToken flag
// distinguishes them.
type GHABootstrapWorkflowPayload struct {
	WorkloadKind string `json:"workload_kind"` // "application" | "compose"
	WorkloadID   string `json:"workload_id"`
	// RotateToken=true mints a fresh deploy token, hashes it, returns
	// the raw value via the result-channel (UI subtab grabs it from
	// the toast). false leaves the existing hash intact — used for
	// "re-sync workflow only" runs in slice I.
	RotateToken bool `json:"rotate_token,omitempty"`
	// LaunchBaseURL captures the externally-reachable URL at dispatch
	// time so the workflow YAML embeds the correct webhook target.
	// Pulled from runtime config when nil, but having it on the
	// payload makes the job self-contained for tests.
	LaunchBaseURL string `json:"launch_base_url,omitempty"`
}

// GHABootstrapWorkflowJob commits the workflow file + writes the
// secret + variables onto the customer's repo via the existing GitHub
// App installation. Persists the resulting commit SHA into
// source_config.gha_workflow_sha so subsequent re-syncs can detect
// drift (someone edited the file in the repo) before re-PUTing.
type GHABootstrapWorkflowJob struct {
	Deps    *JobDeps
	Payload GHABootstrapWorkflowPayload
}

func NewGHABootstrapWorkflowJob(p GHABootstrapWorkflowPayload) pkgjobs.Handler {
	return &GHABootstrapWorkflowJob{Deps: deps, Payload: p}
}

// ghaBootstrapDedupTTL is how long a bootstrap enqueue de-dupes spam-clicks of
// "Re-sync". A self-clearing Unique lock (released when the task completes, or
// after this ttl) is used instead of a fixed asynq.TaskID: a fixed id collides
// permanently with a prior task left in the queue — notably an archived run
// that failed and exhausted retries — which surfaced as a 500
// ("task ID conflicts with another task") and blocked the user from re-syncing.
const ghaBootstrapDedupTTL = 2 * time.Minute

// NewGHABootstrapWorkflowTask is the dispatch helper services call.
// De-duplicated per workload (Unique, self-clearing) so spam-clicking "Re-sync"
// doesn't queue duplicate jobs, while still allowing a fresh re-sync once the
// previous one finishes.
func NewGHABootstrapWorkflowTask(kind, id string, rotate bool, baseURL string) (*asynq.Task, error) {
	return pkgjobs.Task(
		TypeGHABootstrapWorkflow,
		GHABootstrapWorkflowPayload{
			WorkloadKind:  kind,
			WorkloadID:    id,
			RotateToken:   rotate,
			LaunchBaseURL: baseURL,
		},
		asynq.Unique(ghaBootstrapDedupTTL),
	)
}

// Handle does the actual repo writes. Steps:
//
//  1. Load workload (application | compose). Bail if not configured for GHA.
//  2. Decode source_config: repo, branch, installation_id, dockerfile_path.
//  3. If RotateToken (or no existing hash), mint a fresh deploy token.
//  4. Resolve installation token via the GitHub provider.
//  5. PutActionsSecret("LAUNCH_DEPLOY_TOKEN") — sealed-box via GH's pubkey.
//  6. PutActionsVariable("LAUNCH_APP_ID" or "LAUNCH_COMPOSE_ID").
//  7. PutActionsVariable("LAUNCH_WEBHOOK_URL") — base URL the curl posts to.
//  8. Render workflow YAML + PutContents (.github/workflows/launch-deploy.yml).
//  9. Persist the returned commit SHA into source_config.gha_workflow_sha.
//
// On success, broadcast docker.application.gha_synced (or
// .compose.gha_synced) so the UI subtab can refresh.
func (j *GHABootstrapWorkflowJob) Handle(ctx context.Context) error {
	if j.Deps == nil || j.Deps.GitProviders == nil {
		return errors.New("gha bootstrap: git provider factory not wired (set on docker module at boot)")
	}

	switch j.Payload.WorkloadKind {
	case "application":
		return j.handleApplication(ctx)
	case "compose":
		return j.handleCompose(ctx)
	default:
		return fmt.Errorf("gha bootstrap: unsupported workload_kind %q", j.Payload.WorkloadKind)
	}
}

func (j *GHABootstrapWorkflowJob) handleApplication(ctx context.Context) error {
	var app dockermodels.Application
	if err := j.Deps.DB.WithContext(ctx).Where("id = ?", j.Payload.WorkloadID).First(&app).Error; err != nil {
		return fmt.Errorf("gha bootstrap: load application %s: %w", j.Payload.WorkloadID, err)
	}
	if app.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return fmt.Errorf("gha bootstrap: application %s is not configured for github_actions", app.ID)
	}

	cfg, err := parseGHASourceConfig(app.SourceConfig)
	if err != nil {
		return fmt.Errorf("gha bootstrap: application %s: %w", app.ID, err)
	}
	// Carry the chosen builder into the workflow so it's honoured rather
	// than auto-detected. Empty (nil) = auto-detect by Dockerfile
	// presence.
	if app.BuildType != nil {
		cfg.BuildType = string(*app.BuildType)
	}
	// The custom Dockerfile path lives in the application's build_config (set
	// by the create/update flow), NOT source_config — so read it from the row
	// here, mirroring BuildType. Without this the rendered workflow always
	// falls back to the repo-root "Dockerfile" and a custom location like
	// "docker/Dockerfile" is silently ignored.
	cfg.DockerfilePath = resolveAppDockerfilePath(app.BuildConfig, cfg.DockerfilePath)

	// Give this application its OWN GHCR package (#100), so two apps from
	// one repo are isolated into separate packages rather than sharing the
	// repo's package. The package path is `<owner>/<app-slug>`.
	cfg.ImagePackage = imagePackagePath(cfg.Owner, app.Name, app.ID)
	cfg.Platform = j.resolveBuildPlatform(ctx, app.ServerID)

	// Namespace the deploy-token secret and the workflow file per application so
	// multiple apps can share one repo without clobbering each other's token /
	// workflow (which caused cross-app deploy 401s).
	cfg.DeployTokenSecret = appDeployTokenSecretName(app.ID)
	perAppWorkflowPath := appWorkflowPath(app.ID)
	if cfg.WorkflowPath != perAppWorkflowPath {
		// First time on the per-app path (new app, or migrating off the old
		// shared launch-deploy.yml): the stored SHA refers to the old file, so
		// drop it — PutContents resolves the real SHA for the new path itself.
		cfg.ExistingSHA = ""
	}
	cfg.WorkflowPath = perAppWorkflowPath

	// If the deploy token was last pushed under a different secret name (or never
	// recorded — legacy apps migrating to per-app namespacing), the per-app
	// secret doesn't exist yet. A plain re-sync can't copy the old token (only
	// its hash is stored), so force a fresh token mint to populate the new
	// secret — otherwise the per-app workflow references an empty secret → 401.
	lastPushedSecret := stringAt(app.SourceConfig, "gha_deploy_secret_name")
	forceTokenPush := lastPushedSecret != cfg.DeployTokenSecret

	rawToken, tokenHash, err := j.mintTokenIfNeeded(app.GHADeployTokenHash, forceTokenPush)
	if err != nil {
		return err
	}

	// Load build secrets so they can be pushed to GitHub as repo
	// secrets (LAUNCH_BUILD_<NAME>) AND enumerated in the rendered
	// workflow YAML's `secrets:` block. Best-effort: a read failure
	// logs + proceeds with an empty list; the workflow still commits
	// (just without the secrets:) so the customer's pipeline isn't
	// blocked on a transient DB hiccup.
	buildSecrets := j.loadApplicationBuildSecrets(ctx, app.ID)

	if err := j.bootstrap(ctx, cfg, rawToken, "application", app.ID, buildSecrets); err != nil {
		return j.recordBootstrapFailure(ctx, &app, nil, "application", err)
	}

	// Persist tokenHash + workflow SHA + image-repository binding back
	// onto the row. The image repository is what the webhook handler
	// validates incoming `image_tag` against — without it, every GHA
	// deploy notify fails closed at validation.
	// Merge only these keys into source_config at the DB level. A full-column
	// rewrite here raced the auto_deploy toggle that triggers this re-sync —
	// this job loads the row before the toggle commits, so overwriting the
	// whole column dropped auto_deploy. A jsonb `||` merge touches only the
	// keys below and leaves auto_deploy (and anything else) intact.
	scMerge, err := sourceConfigMergeExpr(map[string]any{
		"gha_workflow_sha":       cfg.LastCommitSHA,
		"gha_workflow_path":      cfg.WorkflowPath,
		"gha_deploy_secret_name": cfg.DeployTokenSecret,
		"gha_image_repository":   "ghcr.io/" + cfg.ImagePackage,
		// Success clears any previous failure state so a retry after the
		// customer grants permissions flips the banner off.
		"gha_install_status": string(dockertypes.GHAInstallStatusOK),
		// A successful sync flushes every pending build-secret change.
		"gha_pending_changes": 0,
	})
	if err != nil {
		return fmt.Errorf("gha bootstrap: build source_config merge: %w", err)
	}
	updates := map[string]any{"source_config": scMerge}
	if tokenHash != "" {
		updates["gha_deploy_token_hash"] = tokenHash
	}
	if err := j.Deps.DB.WithContext(ctx).Model(&app).Updates(updates).Error; err != nil {
		return fmt.Errorf("gha bootstrap: persist application %s: %w", app.ID, err)
	}

	j.Deps.Logger.Info().
		Str("application_id", app.ID).
		Str("repo", cfg.Owner+"/"+cfg.Repo).
		Str("workflow_sha", cfg.LastCommitSHA).
		Bool("rotated_token", rawToken != "").
		Msg("gha bootstrap: application workflow synced")

	if j.Deps.Broadcaster != nil {
		j.Deps.Broadcaster.BroadcastToTeam(app.TeamID, "docker.application.gha_synced", map[string]any{
			"application_id": app.ID,
			"workflow_sha":   cfg.LastCommitSHA,
		})
	}
	return nil
}

// isInstallationGone classifies an error as "the GitHub App
// installation no longer exists." Checked at the bootstrap error
// boundary so we can fire a distinct broadcast for the UI's
// reconnection banner.
//
// The GitHub provider currently returns ErrInstallationNotFound for
// the canonical 404-on-installation case; we string-match defensively
// in case the installation-token path bubbles up a different shape.
func isInstallationGone(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gitproviders.ErrInstallationNotFound) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "installation_not_found") ||
		strings.Contains(s, "installation not found") ||
		strings.Contains(s, "status 404")
}

// isPermissionsMissing classifies an error as "the GitHub App is
// installed but lacks one of the permissions bootstrap needs"
// (Contents R/W, Actions Write, Secrets R/W, or Variables R/W).
// GitHub's canonical phrasing is the literal string
// "Resource not accessible by integration" attached to a 403 — we
// match on that substring rather than the status code alone because
// the error chain wraps the raw response body verbatim
// (`PutActionsSecret %s: status %d body %s` in github_writes.go).
//
// Detected at the bootstrap error boundary so we can mark the
// workload non-retriable (asynq otherwise burns ~25 retries against
// a 403 that's never going to succeed), email the creator, and flip
// the UI banner — instead of letting the worker thrash silently
// while the customer wonders why "Setting up" never moves.
func isPermissionsMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gitproviders.ErrPermissionDenied) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "Resource not accessible by integration")
}

// broadcastInstallationBroken fires a team-channel event the UI
// subscribes to. The handler in launch-nuxt's useChannelEvents
// allow-list flips the workload's detail subtab into a "Reconnect
// GitHub App to resume" banner. Idempotent: sending it twice is
// harmless — the UI just refreshes its banner state.
func (j *GHABootstrapWorkflowJob) broadcastInstallationBroken(teamID, kind, id string, cause error) {
	if j.Deps.Broadcaster == nil {
		return
	}
	event := "docker.application.gha_installation_broken"
	idField := "application_id"
	if kind == "compose" {
		event = "docker.compose.gha_installation_broken"
		idField = "compose_id"
	}
	j.Deps.Broadcaster.BroadcastToTeam(teamID, event, map[string]any{
		idField: id,
		"error": cause.Error(),
	})
}

// recordBootstrapFailure is the unified terminal-failure handler for
// the bootstrap pipeline. It classifies the error, persists the
// resulting gha_install_status on the workload's source_config,
// broadcasts the appropriate WS event so the UI flips to an
// actionable banner, sends a notification, and decides whether asynq
// should retry.
//
// Exactly one of (app, compose) must be non-nil. The two paths share
// almost all the work — only the broadcast event name + id field
// differ — so I'd rather one branchy helper than two near-duplicate
// copies that drift.
//
// Return value contract:
//
//   - nil when we've classified the error as a NON-retriable user
//     failure (permissions missing, installation gone). asynq stops
//     retrying; the UI banner is the user's action item.
//   - err propagates when the error is unclassified — likely a real
//     transient (network, DB hiccup). asynq's retry policy then
//     backs off and tries again.
func (j *GHABootstrapWorkflowJob) recordBootstrapFailure(
	ctx context.Context,
	app *dockermodels.Application,
	compose *dockermodels.Compose,
	kind string,
	cause error,
) error {
	var (
		status       dockertypes.GHAInstallStatus
		nonRetriable bool
		workloadID   string
		teamID       string
		workloadName string
		projectID    string
		serverID     string
		sourceConfig map[string]any
		userID       *string
	)

	switch {
	case isPermissionsMissing(cause):
		status, nonRetriable = dockertypes.GHAInstallStatusPermissionsMissing, true
	case isInstallationGone(cause):
		status, nonRetriable = dockertypes.GHAInstallStatusInstallationGone, true
	default:
		// Unclassified — let asynq retry. Don't persist status, the
		// workload stays in "Setting up" and the next attempt will
		// resolve.
		return cause
	}

	if app != nil {
		workloadID, teamID, workloadName = app.ID, app.TeamID, app.Name
		projectID, serverID = app.ProjectID, app.ServerID
		sourceConfig, userID = map[string]any(app.SourceConfig), app.UserID
	} else if compose != nil {
		workloadID, teamID, workloadName = compose.ID, compose.TeamID, compose.Name
		projectID, serverID = compose.ProjectID, compose.ServerID
		sourceConfig, userID = map[string]any(compose.SourceConfig), compose.UserID
	}

	// Persist the status flag on source_config so the UI banner
	// survives reloads. Best-effort — a DB hiccup here logs but
	// shouldn't block the broadcast/email, which are the actionable
	// signals.
	// Merge just the status flag (jsonb ||) so a stale full-column write
	// doesn't drop other source_config keys such as auto_deploy.
	scMerge, mergeErr := sourceConfigMergeExpr(map[string]any{
		"gha_install_status": string(status),
	})
	var persistErr error
	switch {
	case mergeErr != nil:
		persistErr = mergeErr
	case app != nil:
		persistErr = j.Deps.DB.WithContext(ctx).Model(app).
			Update("source_config", scMerge).Error
	default:
		persistErr = j.Deps.DB.WithContext(ctx).Model(compose).
			Update("source_config", scMerge).Error
	}
	if persistErr != nil {
		j.Deps.Logger.Warn().Err(persistErr).
			Str("workload_kind", kind).
			Str("workload_id", workloadID).
			Msg("gha bootstrap: failed to persist gha_install_status; banner won't survive reload")
	}

	// Broadcast. The installation_gone path retains its existing event
	// name for backwards compat with the UI; the new permissions_missing
	// path gets a sibling event with the same payload shape.
	if j.Deps.Broadcaster != nil {
		switch status {
		case dockertypes.GHAInstallStatusInstallationGone:
			j.broadcastInstallationBroken(teamID, kind, workloadID, cause)
		case dockertypes.GHAInstallStatusPermissionsMissing:
			event := "docker.application.gha_permissions_missing"
			idField := "application_id"
			if kind == "compose" {
				event = "docker.compose.gha_permissions_missing"
				idField = "compose_id"
			}
			j.Deps.Broadcaster.BroadcastToTeam(teamID, event, map[string]any{
				idField: workloadID,
				"error": cause.Error(),
			})
		}
	}

	// Email — only for permissions_missing for now. The
	// installation_gone path already has the in-app banner; adding
	// email for the App-uninstalled case can be a follow-up if
	// customers ask. We log enough context that an operator can
	// notice it in the meantime.
	if status == dockertypes.GHAInstallStatusPermissionsMissing {
		j.sendPermissionsMissingNotification(
			ctx, teamID, kind, workloadID, workloadName,
			projectID, serverID, sourceConfig, userID,
		)
	}

	j.Deps.Logger.Warn().Err(cause).
		Str("workload_kind", kind).
		Str("workload_id", workloadID).
		Str("status", string(status)).
		Bool("non_retriable", nonRetriable).
		Msg("gha bootstrap: terminal failure recorded")

	if nonRetriable {
		return nil
	}
	return cause
}

// sendPermissionsMissingNotification routes the
// GHAPermissionsMissingNotification through the team-channel pipe.
// Best-effort: a notifier failure logs + continues so the UI banner
// still shows even if email infra is down.
//
// userID is the workload's creator (migration 0056). Today we still
// fan out via the team channel because the notifier doesn't have a
// SendToUser entry point yet; including the creator's id on the
// payload makes it trivial to swap later. The email body already
// names the workload, so a team member who isn't the creator can
// still hand the message to the App owner.
func (j *GHABootstrapWorkflowJob) sendPermissionsMissingNotification(
	ctx context.Context,
	teamID, kind, workloadID, workloadName, projectID, serverID string,
	sourceConfig map[string]any,
	userID *string,
) {
	if j.Deps.TaskRunnerDeps == nil || j.Deps.TaskRunnerDeps.Notifier == nil {
		return
	}
	notifier := j.Deps.TaskRunnerDeps.Notifier

	projectName := lookupProjectNameForGHA(ctx, j.Deps.DB, teamID, projectID, serverID)
	serverName := lookupServerNameForGHA(ctx, j.Deps.DB, teamID, serverID)
	repository := repositoryFromSourceConfig(sourceConfig)
	appSettingsURL := githubAppSettingsURL(sourceConfig)

	notif := dockernotifications.NewGHAPermissionsMissingNotification(
		kind, workloadName, projectName, serverName,
	).
		WithRepository(repository).
		WithAppSettingsURL(appSettingsURL)

	_ = userID // surfaced for future SendToUser plumbing
	if err := notifier.SendToTeam(ctx, teamID, notif); err != nil {
		j.Deps.Logger.Warn().Err(err).
			Str("workload_kind", kind).
			Str("workload_id", workloadID).
			Msg("gha bootstrap: failed to send permissions-missing notification")
	}
}

// lookupProjectNameForGHA resolves a project name for the notification
// body without coupling to the docker repositories registry (which
// jobs has access to anyway, but this keeps the helper self-contained).
// Returns empty string on any failure — fallback() in the notification
// renders "—" so a missing value doesn't crash the body.
func lookupProjectNameForGHA(ctx context.Context, db *gorm.DB, teamID, projectID, serverID string) string {
	if projectID == "" {
		return ""
	}
	var name string
	err := db.WithContext(ctx).Table("docker_projects").
		Select("name").
		Where("id = ? AND team_id = ? AND server_id = ?", projectID, teamID, serverID).
		Scan(&name).Error
	if err != nil {
		return ""
	}
	return name
}

// lookupServerNameForGHA resolves a server name for the notification.
// Same best-effort posture as lookupProjectNameForGHA.
func lookupServerNameForGHA(ctx context.Context, db *gorm.DB, teamID, serverID string) string {
	if serverID == "" {
		return ""
	}
	var name string
	err := db.WithContext(ctx).Table("servers").
		Select("name").
		Where("id = ? AND team_id = ?", serverID, teamID).
		Scan(&name).Error
	if err != nil {
		return ""
	}
	return name
}

// repositoryFromSourceConfig extracts "owner/repo" for display.
// Prefers explicit owner/repo, falls back to parsing the clone URL.
// Empty when neither path yields a value — the email shows "—".
func repositoryFromSourceConfig(sc map[string]any) string {
	if sc == nil {
		return ""
	}
	owner, _ := sc["owner"].(string)
	repo, _ := sc["repo"].(string)
	if owner != "" && repo != "" {
		return owner + "/" + repo
	}
	// Fall back to whatever's in the repo field (a clone URL or
	// owner/repo) without re-parsing — the email shows it verbatim,
	// human-readable either way.
	if v, ok := sc["repo"].(string); ok {
		return v
	}
	return ""
}

// githubAppSettingsURL gives the user a clickable link to the GitHub
// App's settings page. We don't know the App slug from source_config
// alone (it's set at the provider-config level), so this currently
// returns a generic "where to find your installed Apps" link. A
// future improvement would thread the App slug through via JobDeps.
func githubAppSettingsURL(_ map[string]any) string {
	return "https://github.com/settings/installations"
}

func (j *GHABootstrapWorkflowJob) handleCompose(ctx context.Context) error {
	var compose dockermodels.Compose
	if err := j.Deps.DB.WithContext(ctx).Where("id = ?", j.Payload.WorkloadID).First(&compose).Error; err != nil {
		return fmt.Errorf("gha bootstrap: load compose %s: %w", j.Payload.WorkloadID, err)
	}
	if compose.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return fmt.Errorf("gha bootstrap: compose %s is not configured for github_actions", compose.ID)
	}

	cfg, err := parseGHASourceConfig(compose.SourceConfig)
	if err != nil {
		return fmt.Errorf("gha bootstrap: compose %s: %w", compose.ID, err)
	}

	// Namespace the GHCR image tag per compose stack. The compose template
	// already keys the tag on the service name. Give the whole stack its
	// OWN GHCR package (#100) — `<owner>/<stack-slug>` — so two stacks from
	// one repo are isolated into separate packages; services share the
	// stack's package via per-service tags.
	cfg.ImagePackage = imagePackagePath(cfg.Owner, compose.Name, compose.ID)
	cfg.Platform = j.resolveBuildPlatform(ctx, compose.ServerID)

	// Compose keeps the shared LAUNCH_DEPLOY_TOKEN secret (no per-app
	// namespacing yet), so no forced re-push.
	rawToken, tokenHash, err := j.mintTokenIfNeeded(compose.GHADeployTokenHash, false)
	if err != nil {
		return err
	}

	buildSecrets := j.loadComposeBuildSecrets(ctx, compose.ID)

	if err := j.bootstrap(ctx, cfg, rawToken, "compose", compose.ID, buildSecrets); err != nil {
		return j.recordBootstrapFailure(ctx, nil, &compose, "compose", err)
	}

	// Merge only these keys (jsonb ||) so the re-sync can't clobber a
	// concurrently-set auto_deploy — same fix as the application path.
	scMerge, err := sourceConfigMergeExpr(map[string]any{
		"gha_workflow_sha":     cfg.LastCommitSHA,
		"gha_image_repository": "ghcr.io/" + cfg.ImagePackage,
		"gha_install_status":   string(dockertypes.GHAInstallStatusOK),
		"gha_pending_changes":  0,
	})
	if err != nil {
		return fmt.Errorf("gha bootstrap: build source_config merge: %w", err)
	}
	updates := map[string]any{"source_config": scMerge}
	if tokenHash != "" {
		updates["gha_deploy_token_hash"] = tokenHash
	}
	if err := j.Deps.DB.WithContext(ctx).Model(&compose).Updates(updates).Error; err != nil {
		return fmt.Errorf("gha bootstrap: persist compose %s: %w", compose.ID, err)
	}

	j.Deps.Logger.Info().
		Str("compose_id", compose.ID).
		Str("repo", cfg.Owner+"/"+cfg.Repo).
		Str("workflow_sha", cfg.LastCommitSHA).
		Msg("gha bootstrap: compose workflow synced")

	if j.Deps.Broadcaster != nil {
		j.Deps.Broadcaster.BroadcastToTeam(compose.TeamID, "docker.compose.gha_synced", map[string]any{
			"compose_id":   compose.ID,
			"workflow_sha": cfg.LastCommitSHA,
		})
	}
	return nil
}

// ghaSourceConfig is the decoded shape of the bits of source_config the
// bootstrap job cares about. Source of truth is the JSON column on
// docker_applications / docker_composes — this struct just gives us
// type safety while reading it out.
type ghaSourceConfig struct {
	SourceControlID string
	InstallationID  string
	Owner           string
	Repo            string
	Branch          string
	DockerfilePath  string
	ComposeFilePath string
	WorkflowPath    string
	ExistingSHA     string
	// DeployTokenSecret is the GitHub repo-secret NAME that holds this
	// workload's deploy token. Namespaced per application so multiple apps
	// can share one repo without overwriting each other's token (empty →
	// the shared "LAUNCH_DEPLOY_TOKEN", used by compose).
	DeployTokenSecret string
	// BuildType is the application's chosen builder ("dockerfile" |
	// "nixpacks" | ""). Empty means "auto" — let the workflow detect by
	// Dockerfile presence. A non-empty value is honoured verbatim so an
	// explicit choice isn't silently overridden. Set from the
	// application row in handleApplication (not parsed from
	// source_config); irrelevant for compose (per-service builds).
	BuildType string
	// AutoDeploy, when true, makes the rendered workflow trigger on push
	// to Branch (in addition to manual workflow_dispatch). Persisted in
	// source_config so it survives re-syncs; toggled via the API.
	AutoDeploy bool
	// LastCommitSHA is populated by bootstrap() with the SHA returned
	// from PutContents. Read back into the row's source_config so the
	// next re-sync can detect drift.
	LastCommitSHA string
	// ImagePackage is the GHCR package path (`<owner>/<workload-slug>`)
	// this workload publishes to — its OWN package, so apps/stacks from one
	// repo are isolated into separate packages (#100). Drives both the
	// image tag (`ghcr.io/<ImagePackage>:launch-...`) and the pull-token
	// scope (`repository:<ImagePackage>:pull`). Set from the workload row
	// in handleApplication/handleCompose, not parsed from source_config.
	ImagePackage string
	// Platform is the docker build platform matching the target server's
	// architecture (#101) — "linux/amd64" or "linux/arm64". Resolved from
	// the server's detected_arch in handleApplication/handleCompose.
	Platform string
}

// resolveBuildPlatform maps the target server's detected architecture to a
// docker build platform (#101). Defaults to linux/amd64 when unknown — the
// safe choice for the amd64 GitHub runners and the most common server arch.
func (j *GHABootstrapWorkflowJob) resolveBuildPlatform(ctx context.Context, serverID string) string {
	var row struct{ DetectedArch *string }
	_ = j.Deps.DB.WithContext(ctx).
		Table("servers").
		Select("detected_arch").
		Where("id = ?", serverID).
		Scan(&row).Error
	arch := ""
	if row.DetectedArch != nil {
		arch = *row.DetectedArch
	}
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "arm64", "aarch64":
		return "linux/arm64"
	default:
		return "linux/amd64"
	}
}

// resolveAppDockerfilePath returns the Dockerfile path for an application's
// rendered workflow. The user-chosen path is persisted in the application's
// build_config (not source_config), so a non-empty build_config["dockerfile_path"]
// wins; otherwise the source_config-derived fallback is kept.
func resolveAppDockerfilePath(buildConfig dbtype.JSONMap, fallback string) string {
	if buildConfig != nil {
		if p, ok := buildConfig["dockerfile_path"].(string); ok {
			if p = strings.TrimSpace(p); p != "" {
				return p
			}
		}
	}
	return fallback
}

// appDeployTokenSecretName returns the per-application GitHub repo-secret name
// that holds the deploy token, e.g. "LAUNCH_DEPLOY_TOKEN_01KTK…". Namespacing
// by app id lets several applications share one repo without overwriting each
// other's token (the cause of cross-app deploy 401s). GitHub secret names allow
// [A-Z0-9_] and must not start with a digit — the prefix + uppercased ULID
// satisfies both.
func appDeployTokenSecretName(appID string) string {
	return "LAUNCH_DEPLOY_TOKEN_" + strings.ToUpper(appID)
}

// appWorkflowPath returns the per-application workflow file path. Each app gets
// its own file so apps sharing a repo don't overwrite one another's workflow
// (each embeds its own app id + deploy-token secret reference).
func appWorkflowPath(appID string) string {
	return ".github/workflows/launch-deploy-" + strings.ToLower(appID) + ".yml"
}

func parseGHASourceConfig(raw map[string]any) (*ghaSourceConfig, error) {
	if raw == nil {
		return nil, errors.New("source_config is empty")
	}
	cfg := &ghaSourceConfig{
		SourceControlID: stringAt(raw, "source_control_id"),
		Owner:           stringAt(raw, "owner"),
		Repo:            stringAt(raw, "repo"),
		Branch:          firstNonEmpty(stringAt(raw, "branch"), "main"),
		DockerfilePath:  firstNonEmpty(stringAt(raw, "dockerfile_path"), "Dockerfile"),
		ComposeFilePath: firstNonEmpty(stringAt(raw, "compose_file_path"), "docker-compose.yml"),
		WorkflowPath:    firstNonEmpty(stringAt(raw, "gha_workflow_path"), ".github/workflows/launch-deploy.yml"),
		ExistingSHA:     stringAt(raw, "gha_workflow_sha"),
		AutoDeploy:      boolAt(raw, "auto_deploy"),
	}

	// The application + compose service stores the git source as a
	// single `repo` field that holds the full clone URL (e.g.
	// `git@github.com:owner/name.git` or `https://github.com/owner/name`).
	// Split that into Owner+Repo so the GitHub API helpers (which take
	// them separately) work without forcing a DB shape migration.
	if cfg.Owner == "" || cfg.Repo == "" || looksLikeGitURL(cfg.Repo) {
		if owner, repo, ok := parseRepoIdentifier(stringAt(raw, "repo")); ok {
			cfg.Owner, cfg.Repo = owner, repo
		}
	}
	// Legacy "owner/repo" combined form, still supported.
	if cfg.Owner == "" || cfg.Repo == "" {
		if owner, repo, ok := parseRepoIdentifier(stringAt(raw, "repository")); ok {
			cfg.Owner, cfg.Repo = owner, repo
		}
	}
	if cfg.SourceControlID == "" {
		return nil, errors.New("source_control_id missing from source_config")
	}
	if cfg.Owner == "" || cfg.Repo == "" {
		return nil, errors.New("owner/repo missing from source_config")
	}
	return cfg, nil
}

// ghcrImageRepository is the canonical GHCR image-repository prefix
// the GHA workflow we render publishes images under (it uses
// `ghcr.io/${{ github.repository }}:launch-<sha>`). The webhook
// handler validates the incoming `image_tag` against this value to
// foreclose token-leak → arbitrary-image swaps, so the prefix must be
// lowercased — GHCR rejects mixed-case image refs.
func ghcrImageRepository(owner, repo string) string {
	return "ghcr.io/" + strings.ToLower(owner+"/"+repo)
}

// workloadImageSlug builds the per-workload component of the GHCR image
// tag (`launch-<slug>-...`). It combines a human-readable slug of the
// workload name (the same SlugFromName used for the container name, so the
// two stay consistent) with a short, lowercased fragment of the workload's
// ULID so two workloads in the same repo never collide on one tag — even
// if they share a name — while staying legible on the GHCR package page
// (e.g. "web-3y8r6ck7").
//
// Used for both applications (tag = launch-<slug>-<sha>) and compose stacks
// (tag = launch-<slug>-<service>-<sha>); the id fragment is what stops two
// apps, or two compose stacks sharing a service name, from overwriting each
// other's image.
//
// SlugFromName never returns empty (it falls back to "app"), so the result
// always has a name part; the id fragment guarantees uniqueness.
func workloadImageSlug(name, id string) string {
	idFrag := strings.ToLower(id)
	if len(idFrag) > 8 {
		idFrag = idFrag[len(idFrag)-8:]
	}
	slug := dockertasks.SlugFromName(name)
	if idFrag == "" {
		return slug
	}
	return slug + "-" + idFrag
}

// imagePackagePath is the GHCR package path (the part after "ghcr.io/")
// for a workload — `<owner>/<workload-slug>`, e.g.
// "kkz6/signfly-web-3y8r6ck7". Each application and compose stack gets its
// OWN package so two workloads from one repo are isolated into separate
// packages instead of sharing the repo's package with namespaced tags
// (#100). Lowercased throughout — GHCR rejects mixed-case image refs.
//
// The full image repository is "ghcr.io/" + this; the GHCR pull-token
// scope is "repository:" + this + ":pull".
func imagePackagePath(owner, name, id string) string {
	return strings.ToLower(owner) + "/" + workloadImageSlug(name, id)
}

// looksLikeGitURL is true when s isn't a bare repo name — i.e. it
// contains the scheme/host prefix of a clone URL. We use this to decide
// whether to re-parse the `repo` field of source_config rather than
// trusting it verbatim.
func looksLikeGitURL(s string) bool {
	return strings.Contains(s, "://") || strings.Contains(s, "@") || strings.Contains(s, ":")
}

// parseRepoIdentifier accepts any of the common shapes the docker
// services write into source_config and returns (owner, repo, ok):
//
//	"owner/repo"
//	"git@github.com:owner/repo.git"
//	"https://github.com/owner/repo.git"
//	"https://github.com/owner/repo"
//	"ssh://git@github.com/owner/repo.git"
//
// Returns ok=false when the input is empty or doesn't yield both a
// non-empty owner AND repo. The job validator above turns ok=false into
// a clean "owner/repo missing" error.
func parseRepoIdentifier(raw string) (owner, repo string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", false
	}
	s = strings.TrimSuffix(s, ".git")
	// scp-like ssh: git@github.com:owner/repo
	if strings.HasPrefix(s, "git@") {
		if idx := strings.Index(s, ":"); idx >= 0 {
			s = s[idx+1:]
		}
	}
	// scheme://host/owner/repo
	if i := strings.Index(s, "://"); i >= 0 {
		rest := s[i+3:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			s = rest[slash+1:]
		}
	}
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	owner = strings.TrimSpace(parts[len(parts)-2])
	repo = strings.TrimSpace(parts[len(parts)-1])
	if owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}

func stringAt(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// boolAt reads a bool from the source_config map. JSON unmarshals booleans
// to bool, but tolerate the occasional string ("true") for hand-edited rows.
func boolAt(m map[string]any, key string) bool {
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	default:
		return false
	}
}

func firstNonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// sourceConfigMergeExpr returns a GORM expression that merges patch into the
// source_config JSONB column (source_config || patch) rather than overwriting
// it. A DB-level merge keeps a concurrent writer's keys — notably the
// auto_deploy flag toggled while a workflow re-sync is in flight — from being
// lost to a full-column read-modify-write taken from a stale snapshot.
func sourceConfigMergeExpr(patch map[string]any) (any, error) {
	b, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	return gorm.Expr("COALESCE(source_config, '{}'::jsonb) || ?::jsonb", string(b)), nil
}

// mintTokenIfNeeded generates a fresh deploy token + sha256 hash when rotation
// is requested, force is set, OR the workload has no existing hash. Returns the
// raw plaintext token (empty when not minted) and the hex sha256 to persist.
// Caller takes care of "show raw to user once" upstream.
//
// force is used when the deploy-token secret must be (re)pushed even though a
// hash already exists — e.g. the secret NAME changed (per-app namespacing
// migration). We only hold the hash, not the old plaintext, so we cannot copy
// the existing token to the new secret; a fresh token is the only way to
// populate it. Without this the new per-app workflow would reference a secret
// that was never written → empty token → deploy webhook 401.
func (j *GHABootstrapWorkflowJob) mintTokenIfNeeded(existing *string, force bool) (raw, hash string, err error) {
	needs := force || j.Payload.RotateToken || existing == nil || *existing == ""
	if !needs {
		return "", "", nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("mint deploy token: %w", err)
	}
	raw = hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// bootstrap is the GitHub-side write sequence. Token writes happen
// only when rawToken is non-empty (the rotate path or first-time
// enable); a re-sync without rotation skips the secret PUT.
// buildSecretKV pairs a build-secret name with its plaintext value so
// the bootstrap can both push the value to GitHub (as a repo secret)
// AND render the name into the workflow YAML's `secrets:` block. Lives
// inside this file because no other job needs the shape.
type buildSecretKV struct {
	Name  string
	Value string
}

func (j *GHABootstrapWorkflowJob) bootstrap(
	ctx context.Context,
	cfg *ghaSourceConfig,
	rawToken string,
	workloadKind, workloadID string,
	buildSecrets []buildSecretKV,
) error {
	// Provider lookup: the source_controls row's installation_id is
	// what GetInstallationToken needs. We look up the installation
	// from the database via the source_control_id stored in
	// source_config.
	installationID, err := j.resolveInstallationID(ctx, cfg.SourceControlID)
	if err != nil {
		return err
	}
	cfg.InstallationID = installationID

	gh, err := j.githubProvider()
	if err != nil {
		return err
	}

	// Token secret — only when we're rotating or first-time. The secret name is
	// per-application (cfg.DeployTokenSecret) so apps sharing a repo don't
	// overwrite each other's token; compose leaves it empty → the shared name.
	deployTokenSecret := cfg.DeployTokenSecret
	if deployTokenSecret == "" {
		deployTokenSecret = "LAUNCH_DEPLOY_TOKEN"
	}
	if rawToken != "" {
		if err := gh.PutActionsSecret(ctx, installationID, cfg.Owner, cfg.Repo, deployTokenSecret, rawToken); err != nil {
			return fmt.Errorf("put %s: %w", deployTokenSecret, err)
		}
	}

	// Build-time secrets — push each as LAUNCH_BUILD_<NAME> so the
	// workflow can reference them via ${{ secrets.LAUNCH_BUILD_<NAME> }}
	// in docker/build-push-action's `secrets:` input. We push BEFORE
	// committing the workflow YAML so a successful workflow commit
	// implies the secrets are already in place — no race where a
	// newly-committed YAML references a secret GitHub doesn't know
	// about yet.
	//
	// Failures are not fatal: a single secret-push failure logs +
	// excludes that name from the workflow YAML, so the workflow
	// still commits and the other secrets keep working. The
	// customer sees the missing one as a build-time failure when
	// they next push. The alternative (failing the whole bootstrap)
	// would block all builds for one transient hiccup.
	pushedSecretNames := make([]string, 0, len(buildSecrets))
	for _, s := range buildSecrets {
		repoSecretName := "LAUNCH_BUILD_" + s.Name
		if err := gh.PutActionsSecret(ctx, installationID, cfg.Owner, cfg.Repo, repoSecretName, s.Value); err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("workload_kind", workloadKind).
				Str("workload_id", workloadID).
				Str("repo_secret", repoSecretName).
				Msg("gha bootstrap: failed to push build secret; excluding from workflow")
			continue
		}
		pushedSecretNames = append(pushedSecretNames, s.Name)
	}

	// Variables — overwrite each run (PATCH if exists). Cheap and
	// avoids drift if someone manually edits them in GH's UI.
	idVarName := "LAUNCH_APP_ID"
	if workloadKind == "compose" {
		idVarName = "LAUNCH_COMPOSE_ID"
	}
	if err := gh.PutActionsVariable(ctx, installationID, cfg.Owner, cfg.Repo, idVarName, workloadID); err != nil {
		return fmt.Errorf("put %s variable: %w", idVarName, err)
	}
	if err := gh.PutActionsVariable(ctx, installationID, cfg.Owner, cfg.Repo, "LAUNCH_WEBHOOK_URL", j.Payload.LaunchBaseURL); err != nil {
		return fmt.Errorf("put LAUNCH_WEBHOOK_URL variable: %w", err)
	}

	// Render + commit the workflow YAML. PutContents returns the new
	// commit SHA which we stash on cfg for the caller to persist.
	yaml, err := j.renderWorkflow(cfg, workloadKind, workloadID, pushedSecretNames)
	if err != nil {
		return err
	}
	commitSHA, err := gh.PutContents(
		ctx,
		installationID, cfg.Owner, cfg.Repo,
		cfg.WorkflowPath,
		yaml,
		"Configure Launch deploy workflow",
		cfg.ExistingSHA,
		cfg.Branch,
	)
	if err != nil {
		return fmt.Errorf("put workflow file: %w", err)
	}
	cfg.LastCommitSHA = commitSHA
	return nil
}

// renderWorkflow picks the right template based on workload kind and
// runs slice B's renderer with the resolved field values. buildSecretNames
// drives the optional `secrets:` block on docker/build-push-action — empty
// list produces a clean workflow with no secrets block at all.
func (j *GHABootstrapWorkflowJob) renderWorkflow(
	cfg *ghaSourceConfig, kind, id string, buildSecretNames []string,
) (string, error) {
	switch kind {
	case "application":
		return dockertasks.RenderApplicationWorkflow(dockertasks.ApplicationWorkflowData{
			Branch:            cfg.Branch,
			DockerfilePath:    cfg.DockerfilePath,
			BuildType:         cfg.BuildType,
			LaunchBaseURL:     j.Payload.LaunchBaseURL,
			AppID:             id,
			ImagePackage:      cfg.ImagePackage,
			Platform:          cfg.Platform,
			DeployTokenSecret: cfg.DeployTokenSecret,
			BuildSecretNames:  buildSecretNames,
			AutoDeploy:        cfg.AutoDeploy,
		})
	case "compose":
		return dockertasks.RenderComposeWorkflow(dockertasks.ComposeWorkflowData{
			Branch:           cfg.Branch,
			ComposeFilePath:  cfg.ComposeFilePath,
			LaunchBaseURL:    j.Payload.LaunchBaseURL,
			ComposeID:        id,
			ImagePackage:     cfg.ImagePackage,
			Platform:         cfg.Platform,
			BuildSecretNames: buildSecretNames,
			AutoDeploy:       cfg.AutoDeploy,
		})
	default:
		return "", fmt.Errorf("renderWorkflow: unsupported workload kind %q", kind)
	}
}

// loadApplicationBuildSecrets fetches all live build secrets for the
// application via direct GORM query (the jobs package doesn't have the
// repository registry in JobDeps). Returns plaintext name/value pairs
// — the model column is EncryptedString so GORM decrypts on read.
//
// Best-effort: failure logs + returns empty so the bootstrap commit
// still proceeds (with no secrets:). Tested via the existing bootstrap
// integration; the unit-test surface is the template renderer +
// service layer.
func (j *GHABootstrapWorkflowJob) loadApplicationBuildSecrets(ctx context.Context, appID string) []buildSecretKV {
	var rows []dockermodels.ApplicationBuildSecret
	if err := j.Deps.DB.WithContext(ctx).
		Where("application_id = ?", appID).
		Order("name ASC").
		Find(&rows).Error; err != nil {
		j.Deps.Logger.Warn().Err(err).Str("application_id", appID).
			Msg("gha bootstrap: failed to load build secrets; workflow will commit without them")
		return nil
	}
	out := make([]buildSecretKV, 0, len(rows))
	for _, r := range rows {
		out = append(out, buildSecretKV{Name: r.Name, Value: string(r.Value)})
	}
	return out
}

// loadComposeBuildSecrets is the compose mirror of
// loadApplicationBuildSecrets. Same shape, different table.
func (j *GHABootstrapWorkflowJob) loadComposeBuildSecrets(ctx context.Context, composeID string) []buildSecretKV {
	var rows []dockermodels.ComposeBuildSecret
	if err := j.Deps.DB.WithContext(ctx).
		Where("compose_id = ?", composeID).
		Order("name ASC").
		Find(&rows).Error; err != nil {
		j.Deps.Logger.Warn().Err(err).Str("compose_id", composeID).
			Msg("gha bootstrap: failed to load build secrets; workflow will commit without them")
		return nil
	}
	out := make([]buildSecretKV, 0, len(rows))
	for _, r := range rows {
		out = append(out, buildSecretKV{Name: r.Name, Value: string(r.Value)})
	}
	return out
}

// resolveInstallationID reads the source_controls row to find the
// GitHub App installation we should authenticate as for repo writes.
// Doing this in the job (not the service) keeps the source-control
// lookup off the request path; the bootstrap is one-shot anyway.
func (j *GHABootstrapWorkflowJob) resolveInstallationID(ctx context.Context, sourceControlID string) (string, error) {
	var sc gitmodels.SourceControl
	if err := j.Deps.DB.WithContext(ctx).Where("id = ?", sourceControlID).First(&sc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("source_control %s not found", sourceControlID)
		}
		return "", err
	}
	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return "", fmt.Errorf("source_control %s has no installation_id (GitHub App not installed?)", sourceControlID)
	}
	return *sc.InstallationID, nil
}

func (j *GHABootstrapWorkflowJob) githubProvider() (*gitproviders.GitHubProvider, error) {
	provider, err := j.Deps.GitProviders.GetProvider(gitproviders.GitProviderType(gittypes.GitProviderGitHub))
	if err != nil {
		return nil, fmt.Errorf("resolve github provider: %w", err)
	}
	gh, ok := provider.(*gitproviders.GitHubProvider)
	if !ok {
		return nil, errors.New("provider for github is not a *GitHubProvider")
	}
	return gh, nil
}

// Compile-time guard: ensure we still satisfy the contract-shaped
// dependency that other bits of the codebase might expect from a
// source-control row carrier. Keeps refactors honest.
var _ = gitcontracts.SourceControlService(nil)
