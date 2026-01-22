package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/git/gitref"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
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
		var installationID string
		if idFloat, ok := installation["id"].(float64); ok {
			installationID = fmt.Sprintf("%.0f", idFloat)
		}

		if installationID != "" {
			switch action {
			case "created":
				return j.handleInstallationCreated(ctx, data, installationID)
			case "deleted":
				return j.handleInstallationDeleted(ctx, installationID)
			case "repositories_added", "repositories_removed":
				return j.handleRepositoriesChanged(ctx, installationID)
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

	return j.Deps.Service.GetSourceControlRepo().UpdateFields(ctx, sc.ID, map[string]any{
		"provider_data": providerDataStr,
	})
}

func (j *ProcessGitWebhookJob) handleInstallationDeleted(ctx context.Context, installationID string) error {
	return j.Deps.Service.DeleteByInstallationID(ctx, installationID)
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
	sites, err := j.findSitesByRepositoryAndBranch(ctx, repository, branch)
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

func (j *ProcessGitWebhookJob) findSitesByRepositoryAndBranch(ctx context.Context, repository, branch string) ([]Site, error) {
	// Query sites directly
	var sites []Site
	err := j.Deps.DB.WithContext(ctx).
		Table("sites").
		Select("id, source_control_id").
		Where("repository = ? AND branch = ? AND auto_deployment = ?", repository, branch, true).
		Find(&sites).Error

	return sites, err
}

func (j *ProcessGitWebhookJob) dispatchDeploymentJob(ctx context.Context, site Site, commitData *providers.CommitData) error {
	// TODO: Dispatch deployment job via queue
	// For now, just log
	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("commit_sha", commitData.SHA).
		Str("commit_message", commitData.Message).
		Msg("Would dispatch deployment for site")

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
