package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/git/gitref"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	sitejobs "github.com/kkz6/launch-go/internal/modules/site/jobs"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeProcessGitWebhook = "git:process_webhook"

type ProcessGitWebhookPayload struct {
	Provider  string `json:"provider"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

// ProcessGitWebhookJob processes git webhooks asynchronously
type ProcessGitWebhookJob struct {
	Deps    *JobDeps
	Payload ProcessGitWebhookPayload
}

func NewProcessGitWebhookJob(p ProcessGitWebhookPayload) pkgjobs.Handler {
	return &ProcessGitWebhookJob{Deps: deps, Payload: p}
}

// Handle processes the webhook
func (j *ProcessGitWebhookJob) Handle(ctx context.Context) error {
	providerType, err := gittypes.ParseGitProviderType(j.Payload.Provider)
	if err != nil {
		return fmt.Errorf("invalid provider: %w", err)
	}

	provider, err := j.Deps.ProviderFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// Validate webhook signature
	if !provider.ValidateWebhook([]byte(j.Payload.Payload), j.Payload.Signature) {
		return fmt.Errorf("invalid webhook signature")
	}

	// Parse payload
	var data map[string]any
	if err := json.Unmarshal([]byte(j.Payload.Payload), &data); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	// Process based on provider
	switch providerType {
	case gittypes.GitProviderGitHub:
		return j.processGitHubWebhook(ctx, data)
	case gittypes.GitProviderGitLab:
		return j.processGitLabWebhook(ctx, data)
	case gittypes.GitProviderBitbucket:
		return j.processBitbucketWebhook(ctx, data)
	}

	return nil
}

// Failed is called when the job fails after all retries
func (j *ProcessGitWebhookJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("provider", j.Payload.Provider).
		Msg("Failed to process git webhook")
}

func (j *ProcessGitWebhookJob) processGitHubWebhook(ctx context.Context, data map[string]any) error {
	action, ok := data["action"].(string)
	if !ok && data["action"] != nil {
		j.Deps.Logger.Debug().Interface("action", data["action"]).Msg("GitHub webhook: unexpected action type")
	}

	// Handle installation events
	if installation, ok := data["installation"].(map[string]any); ok {
		installationID := providers.ExtractFloatID(installation, "id")

		if installationID != "" {
			switch action {
			case "created":
				return j.handleInstallationCreated(ctx, data, installationID)
			case "deleted":
				return j.handleInstallationDeleted(ctx, gittypes.GitProviderGitHub, installationID)
			case "repositories_added", "repositories_removed":
				return j.handleRepositoriesChanged(ctx, installationID)
			case "new_permissions_accepted":
				// Owner just granted the new permission set on the App.
				// Re-bootstrap any docker workloads we previously
				// flagged as permissions_missing — they should now
				// succeed and the UI banner clears via the
				// gha_synced broadcast.
				return j.handleGHAPermissionsAccepted(ctx, installationID)
			}
		}
	}

	// Handle push events for deployments
	if commits, ok := data["commits"].([]any); ok && len(commits) > 0 {
		if repository, ok := data["repository"].(map[string]any); ok {
			fullName, ok := repository["full_name"].(string)
			if !ok {
				j.Deps.Logger.Debug().Interface("full_name", repository["full_name"]).Msg("GitHub webhook: unexpected full_name type")
			}
			ref, ok := data["ref"].(string)
			if !ok {
				j.Deps.Logger.Debug().Interface("ref", data["ref"]).Msg("GitHub webhook: unexpected ref type")
			}
			branch := gitref.ExtractBranchName(ref)

			if fullName != "" && branch != "" {
				return j.triggerDeployments(ctx, fullName, branch, data, gittypes.GitProviderGitHub)
			}
		}
	}

	return nil
}

func (j *ProcessGitWebhookJob) processGitLabWebhook(ctx context.Context, data map[string]any) error {
	eventType, ok := data["event_type"].(string)
	if !ok && data["event_type"] != nil {
		j.Deps.Logger.Debug().Interface("event_type", data["event_type"]).Msg("GitLab webhook: unexpected event_type type")
	}

	if eventType == "push" {
		if project, ok := data["project"].(map[string]any); ok {
			fullName, ok := project["path_with_namespace"].(string)
			if !ok {
				j.Deps.Logger.Debug().Interface("path_with_namespace", project["path_with_namespace"]).Msg("GitLab webhook: unexpected path_with_namespace type")
			}
			ref, ok := data["ref"].(string)
			if !ok {
				j.Deps.Logger.Debug().Interface("ref", data["ref"]).Msg("GitLab webhook: unexpected ref type")
			}
			branch := gitref.ExtractBranchName(ref)

			if fullName != "" && branch != "" {
				return j.triggerDeployments(ctx, fullName, branch, data, gittypes.GitProviderGitLab)
			}
		}
	}

	return nil
}

func (j *ProcessGitWebhookJob) processBitbucketWebhook(ctx context.Context, data map[string]any) error {
	if push, ok := data["push"].(map[string]any); ok {
		if changes, ok := push["changes"].([]any); ok && len(changes) > 0 {
			if repository, ok := data["repository"].(map[string]any); ok {
				fullName, ok := repository["full_name"].(string)
				if !ok {
					j.Deps.Logger.Debug().Interface("full_name", repository["full_name"]).Msg("Bitbucket webhook: unexpected full_name type")
				}

				if change, ok := changes[0].(map[string]any); ok {
					var branch string
					if newRef, ok := change["new"].(map[string]any); ok {
						branch, ok = newRef["name"].(string)
						if !ok {
							j.Deps.Logger.Debug().Interface("name", newRef["name"]).Msg("Bitbucket webhook: unexpected branch name type")
						}
					}

					if fullName != "" && branch != "" {
						return j.triggerDeployments(ctx, fullName, branch, data, gittypes.GitProviderBitbucket)
					}
				}
			}
		}
	}

	return nil
}

func (j *ProcessGitWebhookJob) handleInstallationCreated(ctx context.Context, data map[string]any, installationID string) error {
	sender, ok := data["sender"].(map[string]any)
	if !ok {
		j.Deps.Logger.Debug().Interface("sender", data["sender"]).Msg("GitHub webhook: unexpected sender type in installation created")
		return nil
	}
	if sender == nil {
		return nil
	}

	// Find existing source control
	sc, err := j.Deps.Service.GetSourceControlByInstallation(ctx, gittypes.GitProviderGitHub, installationID)
	if err != nil {
		j.Deps.Logger.Warn().Str("installation_id", installationID).Msg("Source control not found for installation")
		return nil
	}

	// Update with installer info
	providerData := make(map[string]any)
	if sc.ProviderData != nil && *sc.ProviderData != "" {
		if err := json.Unmarshal([]byte(*sc.ProviderData), &providerData); err != nil {
			j.Deps.Logger.Warn().Err(err).Str("installation_id", installationID).Msg("Failed to parse existing provider data")
		}
	}

	providerData["github_installer"] = map[string]any{
		"login":                 sender["login"],
		"id":                    sender["id"],
		"type":                  sender["type"],
		"avatar_url":            sender["avatar_url"],
		"installed_via_webhook": true,
	}

	providerDataJSON, err := json.Marshal(providerData)
	if err != nil {
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}
	providerDataStr := string(providerDataJSON)

	return j.Deps.Service.UpdateProviderData(ctx, sc.ID, providerDataStr)
}

func (j *ProcessGitWebhookJob) handleInstallationDeleted(ctx context.Context, provider gittypes.GitProviderType, installationID string) error {
	return j.Deps.Service.DeleteByInstallationID(ctx, provider, installationID)
}

func (j *ProcessGitWebhookJob) handleRepositoriesChanged(ctx context.Context, installationID string) error {
	return j.Deps.Service.SyncRepositoriesForInstallation(ctx, installationID)
}

func (j *ProcessGitWebhookJob) triggerDeployments(ctx context.Context, repository, branch string, webhookData map[string]any, providerType gittypes.GitProviderType) error {
	j.Deps.Logger.Info().
		Str("repository", repository).
		Str("branch", branch).
		Str("provider", providerType.String()).
		Msg("Triggering deployments for repository")

	// Find sites that match this repository and branch
	sites, err := j.findSitesByRepositoryAndBranch(ctx, providerType, repository, branch)
	if err != nil {
		return fmt.Errorf("failed to find sites: %w", err)
	}

	if len(sites) == 0 {
		j.Deps.Logger.Info().
			Str("repository", repository).
			Str("branch", branch).
			Msg("No sites found for auto-deployment")
		return nil
	}

	// Get commit data from webhook
	provider, err := j.Deps.ProviderFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}
	commitData := provider.GetCommitData(webhookData)

	if commitData == nil || commitData.Branch != branch {
		return nil
	}

	// Trigger deployment for each site
	for _, site := range sites {
		if err := j.dispatchDeploymentJob(ctx, site, commitData); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("site_id", site.ID).
				Str("repository", repository).
				Msg("Failed to dispatch deployment")
		}
	}

	return nil
}

// Site represents minimal site data for deployment triggering
type Site struct {
	ID              string
	SourceControlID *string
}

func (j *ProcessGitWebhookJob) findSitesByRepositoryAndBranch(
	ctx context.Context,
	provider gittypes.GitProviderType,
	repository string,
	branch string,
) ([]Site, error) {
	var sites []Site
	err := j.Deps.DB.WithContext(ctx).
		Table("sites").
		Select("sites.id, sites.source_control_id").
		Joins("JOIN source_control_repositories ON source_control_repositories.id = sites.source_control_repositories_id").
		Joins("JOIN source_controls ON source_controls.id = sites.source_control_id").
		Where(
			"source_controls.provider = ? AND source_control_repositories.full_name = ? AND sites.repository_branch = ? AND sites.auto_deployment = ?",
			provider,
			repository,
			branch,
			true,
		).
		Find(&sites).Error

	return sites, err
}

func (j *ProcessGitWebhookJob) dispatchDeploymentJob(ctx context.Context, site Site, commitData *providers.CommitData) error {
	commitMap := commitData.ToMap()
	branch := commitData.Branch

	task, err := sitejobs.NewCreateDeploymentTask(site.ID, nil, nil, &branch, commitMap)
	if err != nil {
		return fmt.Errorf("failed to create deployment task: %w", err)
	}

	if err := j.Deps.DispatchTask(task); err != nil {
		return fmt.Errorf("failed to dispatch deployment task: %w", err)
	}

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("commit_sha", commitData.SHA).
		Str("commit_message", commitData.Message).
		Msg("Dispatched deployment for site")

	return nil
}

// NewProcessGitWebhookTask creates an asynq task for processing a git webhook
func NewProcessGitWebhookTask(provider, payload, signature string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeProcessGitWebhook, ProcessGitWebhookPayload{
		Provider:  provider,
		Payload:   payload,
		Signature: signature,
	})
}

// dockerGHABootstrapTaskType is the canonical asynq task type the
// docker module registers for its bootstrap-workflow handler. Kept as
// a string constant in this file rather than imported from the docker
// jobs package to avoid a cross-module dependency cycle (docker
// already imports git for source-control lookups). The string is part
// of the asynq queue's wire shape — any change to it has to land in
// the docker module first, then here.
const dockerGHABootstrapTaskType = "docker:gha_bootstrap_workflow"

// ghaBootstrapPayload mirrors docker/jobs.GHABootstrapWorkflowPayload's
// JSON shape just closely enough to enqueue a task. Same reason as
// above — we don't import the docker types to avoid a cycle. Keep
// the field tags in sync.
type ghaBootstrapPayload struct {
	WorkloadKind  string `json:"workload_kind"`
	WorkloadID    string `json:"workload_id"`
	RotateToken   bool   `json:"rotate_token,omitempty"`
	LaunchBaseURL string `json:"launch_base_url,omitempty"`
}

// handleGHAPermissionsAccepted is the recovery path for the GitHub
// App permissions-missing failure mode. Fires when the App owner
// accepts a new permission set on an existing installation. We use
// it to re-enqueue the bootstrap for any docker workloads we
// previously marked permissions_missing — they should now succeed,
// the gha_synced broadcast clears the UI banner, and the customer
// doesn't have to click Re-sync manually.
//
// Lives in this package (not docker/) because the GitHub event
// surfaces on the git module's webhook endpoint. We talk to the
// docker tables via raw SQL + enqueue the well-known docker task
// type, both of which avoid importing the docker module (which
// already depends on git, so the reverse import would be a cycle).
func (j *ProcessGitWebhookJob) handleGHAPermissionsAccepted(ctx context.Context, installationID string) error {
	// 1. Find source_controls rows under this installation. There can
	//    be more than one (different teams installed the same App on
	//    different repos) so we collect IDs and pass them through to
	//    the workload lookup.
	var sourceControlIDs []string
	if err := j.Deps.DB.WithContext(ctx).
		Table("source_controls").
		Where("installation_id = ?", installationID).
		Pluck("id", &sourceControlIDs).Error; err != nil {
		return fmt.Errorf("find source_controls for installation %s: %w", installationID, err)
	}
	if len(sourceControlIDs) == 0 {
		j.Deps.Logger.Info().
			Str("installation_id", installationID).
			Msg("gha permissions accepted: no source controls found for installation")
		return nil
	}

	// 2. Find docker_applications + docker_composes under those source
	//    controls that are currently flagged permissions_missing.
	//    JSON ->> lets us read the embedded gha_install_status +
	//    source_control_id without unmarshalling the whole blob.
	type workloadRow struct {
		ID     string
		TeamID string
	}

	var apps []workloadRow
	if err := j.Deps.DB.WithContext(ctx).
		Table("docker_applications").
		Select("id, team_id").
		Where(`build_location = 'github_actions'
			AND source_config->>'gha_install_status' = 'permissions_missing'
			AND source_config->>'source_control_id' IN ?`, sourceControlIDs).
		Where("deleted_at IS NULL").
		Find(&apps).Error; err != nil {
		return fmt.Errorf("find applications to re-bootstrap: %w", err)
	}

	var composes []workloadRow
	if err := j.Deps.DB.WithContext(ctx).
		Table("docker_composes").
		Select("id, team_id").
		Where(`build_location = 'github_actions'
			AND source_config->>'gha_install_status' = 'permissions_missing'
			AND source_config->>'source_control_id' IN ?`, sourceControlIDs).
		Where("deleted_at IS NULL").
		Find(&composes).Error; err != nil {
		return fmt.Errorf("find composes to re-bootstrap: %w", err)
	}

	if len(apps) == 0 && len(composes) == 0 {
		j.Deps.Logger.Info().
			Str("installation_id", installationID).
			Int("source_controls", len(sourceControlIDs)).
			Msg("gha permissions accepted: no permissions_missing workloads to re-bootstrap")
		return nil
	}

	// 3. Enqueue a bootstrap task per workload. Dedup is implicit via
	//    asynq.TaskID — the docker side computes
	//    Dedup("gha_bootstrap_workflow", kind, id) for each task; if
	//    a previous bootstrap for this workload is still in the queue
	//    (rare; it's archived after the non-retriable 403 return)
	//    asynq drops the duplicate harmlessly.
	enqueueOne := func(kind, id string) {
		task, err := newGHABootstrapTask(kind, id)
		if err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("workload_kind", kind).
				Str("workload_id", id).
				Msg("gha permissions accepted: failed to build bootstrap task")
			return
		}
		if err := j.Deps.DispatchTask(task); err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("workload_kind", kind).
				Str("workload_id", id).
				Msg("gha permissions accepted: failed to enqueue bootstrap")
			return
		}
	}

	for _, a := range apps {
		enqueueOne("application", a.ID)
	}
	for _, c := range composes {
		enqueueOne("compose", c.ID)
	}

	j.Deps.Logger.Info().
		Str("installation_id", installationID).
		Int("applications", len(apps)).
		Int("composes", len(composes)).
		Msg("gha permissions accepted: re-bootstrap dispatched for permissions_missing workloads")

	return nil
}

// newGHABootstrapTask builds an asynq task that the docker module's
// bootstrap handler will consume. Mirrors docker/jobs.
// NewGHABootstrapWorkflowTask but lives here to avoid an import
// cycle. RotateToken=false because we're just re-running the
// previously-failed bootstrap — the existing token hash on the
// workload row stays valid as long as it's set.
func newGHABootstrapTask(kind, id string) (*asynq.Task, error) {
	payload, err := json.Marshal(ghaBootstrapPayload{
		WorkloadKind: kind,
		WorkloadID:   id,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal gha bootstrap payload: %w", err)
	}
	// Dedup ID matches the docker module's Dedup() format so two
	// concurrent enqueues (e.g. a webhook AND a manual Re-sync click
	// arriving milliseconds apart) collapse into one execution.
	return asynq.NewTask(
		dockerGHABootstrapTaskType, payload,
		asynq.TaskID(fmt.Sprintf("gha_bootstrap_workflow:%s:%s", kind, id)),
	), nil
}
