package services

import (
	"context"
	"errors"
	"path"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// gha_deploy_dispatch.go houses the synchronous "click Deploy on a GHA
// app → fire workflow_dispatch on the customer's repo" path. Lives
// next to gha_polish.go so all GHA-aware service code is in one
// neighbourhood; called from ApplicationService.Deploy and
// ComposeService.Deploy when build_location=github_actions.
//
// Why synchronous (not an asynq job): the user clicked Deploy and is
// staring at the spinner; a fast inline GitHub call (~200ms) returns
// crisp errors (workflow missing, GitHub down, App uninstalled) that
// the UI can show RIGHT NOW. Queueing the dispatch would force us to
// invent a separate WS event just to surface "your dispatch failed,"
// which is worse UX than failing the POST.

// ghaDispatchFields is the minimal shape Deploy() needs out of a
// workload's source_config to fire a workflow_dispatch. We re-derive
// it from the live source_config every call (cheap; just string
// reads) rather than caching to avoid stale-after-rename bugs.
type ghaDispatchFields struct {
	SourceControlID string
	Owner           string
	Repo            string
	Branch          string
	WorkflowFile    string // basename, e.g. "launch-deploy.yml"
	WorkflowSHA     string // empty == bootstrap not finished yet
}

// parseGHADispatchFields pulls the dispatch-relevant fields out of a
// source_config blob. Returns a validation error if anything required
// is missing — the caller maps that to fiberutil.Conflict /
// fiberutil.Validation as appropriate.
//
// Mirrors (but doesn't share) jobs.parseGHASourceConfig so the
// services package doesn't reach into a jobs internal. Drift risk is
// bounded — both readers consume the same source_config keys that
// applicationService.Create / composeService.Create write at boot.
func parseGHADispatchFields(raw map[string]any) (*ghaDispatchFields, error) {
	if raw == nil {
		return nil, errors.New("source_config is empty")
	}

	get := func(k string) string {
		v, _ := raw[k].(string)
		return v
	}

	fields := &ghaDispatchFields{
		SourceControlID: get("source_control_id"),
		Owner:           get("owner"),
		Repo:            get("repo"),
		Branch:          firstNonEmptyDispatch(get("branch"), "main"),
		WorkflowSHA:     get("gha_workflow_sha"),
	}

	// workflow_path is the full path under the repo
	// (.github/workflows/launch-deploy.yml). GitHub's
	// workflow_dispatch endpoint takes just the basename, so strip
	// the dir. Default matches the bootstrap default.
	workflowPath := firstNonEmptyDispatch(get("gha_workflow_path"), ".github/workflows/launch-deploy.yml")
	fields.WorkflowFile = path.Base(workflowPath)

	// Owner/Repo are also stored — when they are not present, fall
	// back to parsing them out of the legacy "repo" field which
	// holds a clone URL. Mirrors what bootstrap does so an app
	// created before owner/repo were split still dispatches cleanly.
	if fields.Owner == "" || fields.Repo == "" {
		if o, r, ok := parseRepoIdentifierDispatch(get("repo")); ok {
			fields.Owner, fields.Repo = o, r
		}
	}
	if fields.Owner == "" || fields.Repo == "" {
		if o, r, ok := parseRepoIdentifierDispatch(get("repository")); ok {
			fields.Owner, fields.Repo = o, r
		}
	}

	if fields.SourceControlID == "" {
		return nil, errors.New("source_control_id missing from source_config")
	}
	if fields.Owner == "" || fields.Repo == "" {
		return nil, errors.New("owner/repo missing from source_config")
	}
	return fields, nil
}

func firstNonEmptyDispatch(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// parseRepoIdentifierDispatch accepts any of the common shapes
// docker/git source_config stores (bare "owner/repo", scp-style ssh
// "git@github.com:owner/repo.git", https URL) and returns
// (owner, repo, ok). Mirrors the jobs-package helper but kept local
// so services doesn't import jobs internals.
func parseRepoIdentifierDispatch(raw string) (owner, repo string, ok bool) {
	s := raw
	if s == "" {
		return "", "", false
	}
	// scp-like ssh: git@github.com:owner/repo
	if at := indexOf(s, "@"); at >= 0 {
		if colon := indexOfAfter(s, ":", at); colon >= 0 {
			s = s[colon+1:]
		}
	}
	// scheme://host/owner/repo
	if i := indexOf(s, "://"); i >= 0 {
		rest := s[i+3:]
		if slash := indexOf(rest, "/"); slash >= 0 {
			s = rest[slash+1:]
		}
	}
	s = trimSuffix(s, ".git")
	parts := splitAll(s, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	owner = parts[len(parts)-2]
	repo = parts[len(parts)-1]
	if owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}

// Tiny string helpers — keeping the parser self-contained (no extra
// imports of the strings package — that's already imported elsewhere
// in this package, but inlining keeps this file scan-readable).
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
func indexOfAfter(s, sub string, after int) int {
	for i := after + 1; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
func trimSuffix(s, suf string) string {
	if len(s) >= len(suf) && s[len(s)-len(suf):] == suf {
		return s[:len(s)-len(suf)]
	}
	return s
}
func splitAll(s, sep string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			out = append(out, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	out = append(out, s[start:])
	return out
}

// resolveGitHubInstallationID looks up the GitHub App installation
// id for a given source_controls row. Same shape as the jobs-package
// helper but kept local so the service path doesn't depend on jobs
// internals.
func resolveGitHubInstallationID(ctx context.Context, db *gorm.DB, sourceControlID string) (string, error) {
	var sc gitmodels.SourceControl
	if err := db.WithContext(ctx).Where("id = ?", sourceControlID).First(&sc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fiberutil.Validation("GitHub connection for this app was deleted; reconnect from Settings → Connections.")
		}
		return "", err
	}
	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return "", fiberutil.Validation("GitHub App is no longer installed on this account; reinstall from Settings → Connections.")
	}
	return *sc.InstallationID, nil
}

// ghaWorkflowDispatcher is the subset of *gitproviders.GitHubProvider
// the dispatch path needs. Defined as an interface so unit tests can
// supply a fake without a real HTTP client.
type ghaWorkflowDispatcher interface {
	TriggerWorkflowDispatch(ctx context.Context, installationID, owner, repo, workflowFile, branch string) error
	DeleteWorkflowRun(ctx context.Context, installationID, owner, repo, runID string) error
}

// deleteGHAWorkflowRun removes a GitHub Actions run for a workload, resolving
// owner/repo/installation from the workload's source_config. Used by the
// delete-deployment flow when the user opts to also remove the run from GitHub.
func deleteGHAWorkflowRun(
	ctx context.Context,
	db *gorm.DB,
	factory *gitproviders.ProviderFactory,
	sourceConfig map[string]any,
	runID string,
) error {
	fields, err := parseGHADispatchFields(sourceConfig)
	if err != nil {
		return err
	}
	installationID, err := resolveGitHubInstallationID(ctx, db, fields.SourceControlID)
	if err != nil {
		return err
	}
	gh, err := resolveGitHubDispatcher(factory)
	if err != nil {
		return err
	}
	return gh.DeleteWorkflowRun(ctx, installationID, fields.Owner, fields.Repo, runID)
}

// resolveGitHubDispatcher returns the GitHub provider as the slim
// ghaWorkflowDispatcher interface, or an error if the provider
// factory is unwired (dev rig) or doesn't expose a GitHub provider.
// Caller surfaces the error as a clear "GitHub provider not
// configured" message rather than a 500.
func resolveGitHubDispatcher(factory *gitproviders.ProviderFactory) (ghaWorkflowDispatcher, error) {
	if factory == nil {
		return nil, fiberutil.Validation("GitHub provider is not configured on this Launch deployment; ask an admin to configure the GitHub App.")
	}
	provider, err := factory.GetProvider(gitproviders.GitProviderType(gittypes.GitProviderGitHub))
	if err != nil {
		return nil, fiberutil.Validation("GitHub provider is not configured on this Launch deployment; ask an admin to configure the GitHub App.")
	}
	gh, ok := provider.(*gitproviders.GitHubProvider)
	if !ok {
		return nil, errors.New("git provider factory returned a non-GitHub provider for the github type")
	}
	return gh, nil
}

// dispatchGHADeploy is the shared work both ApplicationService.Deploy
// and ComposeService.Deploy do when build_location=github_actions:
// validate that bootstrap has finished, parse owner/repo/branch out
// of source_config, resolve the installation token, and fire
// workflow_dispatch on the customer's repo.
//
// Returns nil on success; any error is already wrapped as a
// fiberutil 4xx where appropriate so the route handler can return it
// verbatim.
func dispatchGHADeploy(
	ctx context.Context,
	db *gorm.DB,
	gh ghaWorkflowDispatcher,
	sourceConfig map[string]any,
) error {
	fields, err := parseGHADispatchFields(sourceConfig)
	if err != nil {
		return fiberutil.Validation(err.Error())
	}
	if fields.WorkflowSHA == "" {
		return fiberutil.Conflict("GitHub Actions setup hasn't finished. Wait for the bootstrap to commit the workflow file, or click Re-sync workflow on the GitHub Actions tab.")
	}

	installationID, err := resolveGitHubInstallationID(ctx, db, fields.SourceControlID)
	if err != nil {
		return err
	}

	if err := gh.TriggerWorkflowDispatch(
		ctx, installationID, fields.Owner, fields.Repo, fields.WorkflowFile, fields.Branch,
	); err != nil {
		if errors.Is(err, gitproviders.ErrWorkflowNotFound) {
			return fiberutil.Conflict(
				"The workflow file is missing from the repo. Click Re-sync workflow on the GitHub Actions tab to recommit it.",
			)
		}
		if errors.Is(err, gitproviders.ErrInstallationNotFound) ||
			errors.Is(err, gitproviders.ErrPermissionDenied) {
			return fiberutil.Validation(
				"GitHub App lost access to this repo. Reinstall the GitHub App or grant repo access from Settings → Connections.",
			)
		}
		return err
	}
	return nil
}

// deployViaGitHubActions is ApplicationService's GHA branch — called
// from Deploy() when app.BuildLocation == github_actions. Fires the
// workflow on GitHub, then records a placeholder pending deployment
// row so the UI shows immediate movement. The real "deploying"
// deployment row (with gha_run_id, image tag, etc.) is created by
// the existing webhook handler when GitHub Actions notifies us back.
func (s *ApplicationService) deployViaGitHubActions(
	ctx context.Context,
	app *models.Application,
	serverID, teamID string,
) (*models.Deployment, error) {
	gh, err := resolveGitHubDispatcher(s.GitProviders())
	if err != nil {
		return nil, err
	}
	if err := dispatchGHADeploy(ctx, s.ServiceDeps().DB, gh, map[string]any(app.SourceConfig)); err != nil {
		return nil, err
	}

	// Placeholder row for the UI. trigger_source=github_actions
	// distinguishes it from a server-side build (the deployment-list
	// "via GitHub Actions" badge keys on this). The webhook later
	// creates the REAL row for the eventual GHA run.
	now := time.Now().UTC()
	deployment := &models.Deployment{
		TargetType:    "application",
		TargetID:      app.ID,
		Status:        dockertypes.DeploymentStatusPending,
		StartedAt:     &now,
		TriggerSource: dockertypes.DeploymentTriggerGitHubActions,
	}
	deployment.TeamID = teamID
	deployment.ServerID = serverID
	if err := s.Repos().Deployment().Create(ctx, deployment); err != nil {
		// Don't fail the user request — the dispatch already fired
		// and the webhook will create its own row. Log + continue.
		s.LogError(err, "create placeholder GHA deployment row", "application_id", app.ID)
	}

	s.BroadcastToTeam(teamID, "docker.application.deploying", map[string]any{
		"application_id": app.ID,
		"deployment_id":  deployment.ID,
		"server_id":      serverID,
		"team_id":        teamID,
		"status":         "pending",
		"trigger_source": string(dockertypes.DeploymentTriggerGitHubActions),
	})
	return deployment, nil
}

// deployViaGitHubActions is ComposeService's GHA branch — same shape
// as the application path, different target_type + broadcast event.
func (s *ComposeService) deployViaGitHubActions(
	ctx context.Context,
	c *models.Compose,
	serverID, teamID string,
) (*models.Deployment, error) {
	gh, err := resolveGitHubDispatcher(s.GitProviders())
	if err != nil {
		return nil, err
	}
	if err := dispatchGHADeploy(ctx, s.ServiceDeps().DB, gh, map[string]any(c.SourceConfig)); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	deployment := &models.Deployment{
		TargetType:    "compose",
		TargetID:      c.ID,
		Status:        dockertypes.DeploymentStatusPending,
		StartedAt:     &now,
		TriggerSource: dockertypes.DeploymentTriggerGitHubActions,
	}
	deployment.TeamID = teamID
	deployment.ServerID = serverID
	if err := s.Repos().Deployment().Create(ctx, deployment); err != nil {
		s.LogError(err, "create placeholder GHA compose deployment row", "compose_id", c.ID)
	}

	s.BroadcastToTeam(teamID, "docker.compose.deploying", map[string]any{
		"compose_id":     c.ID,
		"deployment_id":  deployment.ID,
		"server_id":      serverID,
		"team_id":        teamID,
		"status":         "pending",
		"trigger_source": string(dockertypes.DeploymentTriggerGitHubActions),
	})
	return deployment, nil
}
