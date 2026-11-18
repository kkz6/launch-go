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
	Provider   string `json:"provider"`
	Payload    string `json:"payload"`
	Signature  string `json:"signature"`
	Event      string `json:"event"`
	DeliveryID string `json:"delivery_id"`
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
	return j.processVerifiedGitHubEvent(ctx, data)
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
		Joins("JOIN source_controls ON source_controls.id = sites.source_control_id AND source_controls.team_id = sites.team_id AND source_control_repositories.source_control_id = source_controls.id").
		Where(
			"source_controls.provider = ? AND source_control_repositories.full_name = ? AND COALESCE(NULLIF(sites.repository_branch, ''), 'main') = ? AND sites.auto_deployment = ?",
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
func NewProcessGitWebhookTask(provider, payload, signature string, headers ...string) (*asynq.Task, error) {
	var event, deliveryID string
	if len(headers) == 2 {
		event, deliveryID = headers[0], headers[1]
	}
	return pkgjobs.Task(TypeProcessGitWebhook, ProcessGitWebhookPayload{
		Provider:  provider,
		Payload:   payload,
		Signature: signature,
		Event:     event, DeliveryID: deliveryID,
	}, asynq.MaxRetry(10))
}
