package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// ProcessGitWebhookJob processes git webhooks asynchronously
type ProcessGitWebhookJob struct {
	db              *gorm.DB
	logger          *zerolog.Logger
	service         *services.SourceControlService
	providerFactory *providers.ProviderFactory
	siteRepo        SiteRepository
	queueClient     QueueClient
	Payload         ProcessGitWebhookPayload
}

// SiteRepository interface for site operations
type SiteRepository interface {
	FindByRepositoryAndBranch(ctx context.Context, repository, branch string) ([]any, error)
}

// QueueClient interface for dispatching jobs
type QueueClient interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Type returns the job type identifier
func (j *ProcessGitWebhookJob) Type() string {
	return TypeProcessGitWebhook
}

// Handle processes the webhook
func (j *ProcessGitWebhookJob) Handle(ctx context.Context) error {
	providerType, err := enums.ParseGitProviderType(j.Payload.Provider)
	if err != nil {
		return fmt.Errorf("invalid provider: %w", err)
	}

	provider, err := j.providerFactory.GetProvider(providers.GitProviderType(providerType))
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
	case enums.GitProviderGitHub:
		return j.processGitHubWebhook(ctx, data)
	case enums.GitProviderGitLab:
		return j.processGitLabWebhook(ctx, data)
	case enums.GitProviderBitbucket:
		return j.processBitbucketWebhook(ctx, data)
	}

	return nil
}

// Failed is called when the job fails after all retries
func (j *ProcessGitWebhookJob) Failed(ctx context.Context, err error) {
	j.logger.Error().Err(err).
		Str("provider", j.Payload.Provider).
		Msg("Failed to process git webhook")
}

func (j *ProcessGitWebhookJob) processGitHubWebhook(ctx context.Context, data map[string]any) error {
	action, _ := data["action"].(string)

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
			fullName, _ := repository["full_name"].(string)
			ref, _ := data["ref"].(string)
			branch := strings.TrimPrefix(ref, "refs/heads/")

			if fullName != "" && branch != "" {
				return j.triggerDeployments(ctx, fullName, branch, data, enums.GitProviderGitHub)
			}
		}
	}

	return nil
}

func (j *ProcessGitWebhookJob) processGitLabWebhook(ctx context.Context, data map[string]any) error {
	eventType, _ := data["event_type"].(string)

	if eventType == "push" {
		if project, ok := data["project"].(map[string]any); ok {
			fullName, _ := project["path_with_namespace"].(string)
			ref, _ := data["ref"].(string)
			branch := strings.TrimPrefix(ref, "refs/heads/")

			if fullName != "" && branch != "" {
				return j.triggerDeployments(ctx, fullName, branch, data, enums.GitProviderGitLab)
			}
		}
	}

	return nil
}

func (j *ProcessGitWebhookJob) processBitbucketWebhook(ctx context.Context, data map[string]any) error {
	if push, ok := data["push"].(map[string]any); ok {
		if changes, ok := push["changes"].([]any); ok && len(changes) > 0 {
			if repository, ok := data["repository"].(map[string]any); ok {
				fullName, _ := repository["full_name"].(string)

				if change, ok := changes[0].(map[string]any); ok {
					var branch string
					if newRef, ok := change["new"].(map[string]any); ok {
						branch, _ = newRef["name"].(string)
					}

					if fullName != "" && branch != "" {
						return j.triggerDeployments(ctx, fullName, branch, data, enums.GitProviderBitbucket)
					}
				}
			}
		}
	}

	return nil
}

func (j *ProcessGitWebhookJob) handleInstallationCreated(ctx context.Context, data map[string]any, installationID string) error {
	sender, _ := data["sender"].(map[string]any)
	if sender == nil {
		return nil
	}

	// Find existing source control
	sc, err := j.service.GetSourceControlByInstallation(ctx, enums.GitProviderGitHub, installationID)
	if err != nil {
		j.logger.Warn().Str("installation_id", installationID).Msg("Source control not found for installation")
		return nil
	}

	// Update with installer info
	providerData := make(map[string]any)
	if sc.ProviderData != nil && *sc.ProviderData != "" {
		_ = json.Unmarshal([]byte(*sc.ProviderData), &providerData)
	}

	providerData["github_installer"] = map[string]any{
		"login":                 sender["login"],
		"id":                    sender["id"],
		"type":                  sender["type"],
		"avatar_url":            sender["avatar_url"],
		"installed_via_webhook": true,
	}

	providerDataJSON, _ := json.Marshal(providerData)
	providerDataStr := string(providerDataJSON)

	return j.service.GetSourceControlRepo().UpdateFields(ctx, sc.ID, map[string]any{
		"provider_data": providerDataStr,
	})
}

func (j *ProcessGitWebhookJob) handleInstallationDeleted(ctx context.Context, installationID string) error {
	return j.service.DeleteByInstallationID(ctx, installationID)
}

func (j *ProcessGitWebhookJob) handleRepositoriesChanged(ctx context.Context, installationID string) error {
	return j.service.SyncRepositoriesForInstallation(ctx, installationID)
}

func (j *ProcessGitWebhookJob) triggerDeployments(ctx context.Context, repository, branch string, webhookData map[string]any, providerType enums.GitProviderType) error {
	j.logger.Info().
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
		j.logger.Info().
			Str("repository", repository).
			Str("branch", branch).
			Msg("No sites found for auto-deployment")
		return nil
	}

	// Get commit data from webhook
	provider, _ := j.providerFactory.GetProvider(providers.GitProviderType(providerType))
	commitData := provider.GetCommitData(webhookData)

	if commitData == nil || commitData.Branch != branch {
		return nil
	}

	// Trigger deployment for each site
	for _, site := range sites {
		if err := j.dispatchDeploymentJob(ctx, site, commitData); err != nil {
			j.logger.Error().Err(err).
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
	err := j.db.WithContext(ctx).
		Table("sites").
		Select("id, source_control_id").
		Where("repository = ? AND branch = ? AND auto_deployment = ?", repository, branch, true).
		Find(&sites).Error

	return sites, err
}

func (j *ProcessGitWebhookJob) dispatchDeploymentJob(ctx context.Context, site Site, commitData *providers.CommitData) error {
	// TODO: Dispatch deployment job via queue
	// For now, just log
	j.logger.Info().
		Str("site_id", site.ID).
		Str("commit_sha", commitData.SHA).
		Str("commit_message", commitData.Message).
		Msg("Would dispatch deployment for site")

	return nil
}

// NewProcessGitWebhookTask creates an asynq task for processing a git webhook
func NewProcessGitWebhookTask(provider, payload, signature string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeProcessGitWebhook, ProcessGitWebhookPayload{
		Provider:  provider,
		Payload:   payload,
		Signature: signature,
	})
}

// JobContext holds dependencies for git jobs
type JobContext struct {
	DB              *gorm.DB
	Logger          *zerolog.Logger
	Service         *services.SourceControlService
	ProviderFactory *providers.ProviderFactory
	SCRepo          *repositories.SourceControlRepository
	QueueClient     QueueClient
}

// NewProcessGitWebhookJob creates a new ProcessGitWebhookJob
func NewProcessGitWebhookJob(ctx *JobContext, payload ProcessGitWebhookPayload) *ProcessGitWebhookJob {
	return &ProcessGitWebhookJob{
		db:              ctx.DB,
		logger:          ctx.Logger,
		service:         ctx.Service,
		providerFactory: ctx.ProviderFactory,
		Payload:         payload,
	}
}
