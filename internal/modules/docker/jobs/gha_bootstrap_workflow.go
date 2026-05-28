package jobs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	gitcontracts "github.com/kkz6/launch-go/internal/modules/git/contracts"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"

	"gorm.io/gorm"

	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertasks "github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
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

// NewGHABootstrapWorkflowTask is the dispatch helper services call.
// Deduplicated per workload so spam-clicking "Re-sync" doesn't queue
// duplicate jobs.
func NewGHABootstrapWorkflowTask(kind, id string, rotate bool, baseURL string) (*asynq.Task, error) {
	return pkgjobs.Task(
		TypeGHABootstrapWorkflow,
		GHABootstrapWorkflowPayload{
			WorkloadKind:  kind,
			WorkloadID:    id,
			RotateToken:   rotate,
			LaunchBaseURL: baseURL,
		},
		asynq.TaskID(pkgjobs.Dedup("gha_bootstrap_workflow", kind, id)),
	)
}

// Handle does the actual repo writes. Steps:
//
//	1. Load workload (application | compose). Bail if not configured for GHA.
//	2. Decode source_config: repo, branch, installation_id, dockerfile_path.
//	3. If RotateToken (or no existing hash), mint a fresh deploy token.
//	4. Resolve installation token via the GitHub provider.
//	5. PutActionsSecret("LAUNCH_DEPLOY_TOKEN") — sealed-box via GH's pubkey.
//	6. PutActionsVariable("LAUNCH_APP_ID" or "LAUNCH_COMPOSE_ID").
//	7. PutActionsVariable("LAUNCH_WEBHOOK_URL") — base URL the curl posts to.
//	8. Render workflow YAML + PutContents (.github/workflows/launch-deploy.yml).
//	9. Persist the returned commit SHA into source_config.gha_workflow_sha.
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

	rawToken, tokenHash, err := j.mintTokenIfNeeded(app.GHADeployTokenHash)
	if err != nil {
		return err
	}

	if err := j.bootstrap(ctx, cfg, rawToken, "application", app.ID); err != nil {
		// Special-case the "GitHub App installation removed" failure
		// mode so the UI can render a banner the customer can actually
		// act on, rather than the user staring at a generic asynq
		// retry loop in Sentry. installationGone is true when our
		// installation token request 404s — meaning the customer
		// uninstalled the GitHub App from their account.
		if isInstallationGone(err) {
			j.broadcastInstallationBroken(app.TeamID, "application", app.ID, err)
		}
		return err
	}

	// Persist tokenHash + workflow SHA back onto the row.
	updates := map[string]any{
		"source_config": appendSourceConfig(app.SourceConfig, map[string]any{
			"gha_workflow_sha": cfg.LastCommitSHA,
		}),
	}
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

	rawToken, tokenHash, err := j.mintTokenIfNeeded(compose.GHADeployTokenHash)
	if err != nil {
		return err
	}

	if err := j.bootstrap(ctx, cfg, rawToken, "compose", compose.ID); err != nil {
		if isInstallationGone(err) {
			j.broadcastInstallationBroken(compose.TeamID, "compose", compose.ID, err)
		}
		return err
	}

	updates := map[string]any{
		"source_config": appendSourceConfig(compose.SourceConfig, map[string]any{
			"gha_workflow_sha": cfg.LastCommitSHA,
		}),
	}
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
	// LastCommitSHA is populated by bootstrap() with the SHA returned
	// from PutContents. Read back into the row's source_config so the
	// next re-sync can detect drift.
	LastCommitSHA string
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
	}

	// Repository may have been saved as "owner/repo" rather than as
	// separate fields. Parse it apart for the API helpers.
	if cfg.Owner == "" || cfg.Repo == "" {
		if combined := stringAt(raw, "repository"); combined != "" {
			for i := 0; i < len(combined); i++ {
				if combined[i] == '/' {
					cfg.Owner = combined[:i]
					cfg.Repo = combined[i+1:]
					break
				}
			}
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

func stringAt(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func firstNonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// appendSourceConfig returns a NEW map with updates merged in. Doesn't
// mutate the original — GORM JSON serialization needs a fresh map.
func appendSourceConfig(orig map[string]any, updates map[string]any) map[string]any {
	out := make(map[string]any, len(orig)+len(updates))
	for k, v := range orig {
		out[k] = v
	}
	for k, v := range updates {
		out[k] = v
	}
	return out
}

// mintTokenIfNeeded generates a fresh deploy token + sha256 hash when
// rotation is requested OR the workload has no existing hash. Returns
// the raw plaintext token (empty when not rotated) and the hex sha256
// to persist. Caller takes care of "show raw to user once" upstream.
func (j *GHABootstrapWorkflowJob) mintTokenIfNeeded(existing *string) (raw, hash string, err error) {
	needs := j.Payload.RotateToken || existing == nil || *existing == ""
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
func (j *GHABootstrapWorkflowJob) bootstrap(
	ctx context.Context,
	cfg *ghaSourceConfig,
	rawToken string,
	workloadKind, workloadID string,
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

	// Token secret — only when we're rotating or first-time.
	if rawToken != "" {
		if err := gh.PutActionsSecret(ctx, installationID, cfg.Owner, cfg.Repo, "LAUNCH_DEPLOY_TOKEN", rawToken); err != nil {
			return fmt.Errorf("put LAUNCH_DEPLOY_TOKEN: %w", err)
		}
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
	yaml, err := j.renderWorkflow(cfg, workloadKind, workloadID)
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
// runs slice B's renderer with the resolved field values.
func (j *GHABootstrapWorkflowJob) renderWorkflow(cfg *ghaSourceConfig, kind, id string) (string, error) {
	switch kind {
	case "application":
		return dockertasks.RenderApplicationWorkflow(dockertasks.ApplicationWorkflowData{
			Branch:         cfg.Branch,
			DockerfilePath: cfg.DockerfilePath,
			LaunchBaseURL:  j.Payload.LaunchBaseURL,
			AppID:          id,
		})
	case "compose":
		return dockertasks.RenderComposeWorkflow(dockertasks.ComposeWorkflowData{
			Branch:          cfg.Branch,
			ComposeFilePath: cfg.ComposeFilePath,
			LaunchBaseURL:   j.Payload.LaunchBaseURL,
			ComposeID:       id,
		})
	default:
		return "", fmt.Errorf("renderWorkflow: unsupported workload kind %q", kind)
	}
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
