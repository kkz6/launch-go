package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSyncInstallationRepos = "git:sync_installation_repos"

type SyncInstallationReposPayload struct {
	Provider       string `json:"provider"`
	InstallationID string `json:"installation_id"`
	TeamID         string `json:"team_id"`
	UserID         string `json:"user_id"`
}

// SyncInstallationReposJob syncs repositories for a git installation
type SyncInstallationReposJob struct {
	ctx     *JobContext
	Payload SyncInstallationReposPayload
}

// Handle processes the job
func (j *SyncInstallationReposJob) Handle(ctx context.Context) error {
	providerType, err := gittypes.ParseGitProviderType(j.Payload.Provider)
	if err != nil {
		return fmt.Errorf("invalid provider: %w", err)
	}

	// Find source control for this installation
	sc, err := j.ctx.Service.GetSourceControlByInstallation(ctx, providerType, j.Payload.InstallationID)
	if err != nil {
		j.ctx.Logger.Warn().
			Str("installation_id", j.Payload.InstallationID).
			Msg("Source control not found for installation")
		return nil
	}

	// Get provider
	provider, err := j.ctx.ProviderFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// Get repositories from provider
	repositories, err := provider.GetInstallationRepositories(ctx, j.Payload.InstallationID)
	if err != nil {
		return fmt.Errorf("failed to get repositories: %w", err)
	}

	// Sync repositories
	if err := j.ctx.Service.SyncInstallationRepositories(ctx, sc, repositories); err != nil {
		return fmt.Errorf("failed to sync repositories: %w", err)
	}

	// Update repository count
	if err := j.ctx.SCRepo.UpdateFields(ctx, sc.ID, map[string]any{
		"repository_count": len(repositories),
	}); err != nil {
		j.ctx.Logger.Warn().Err(err).Msg("Failed to update repository count")
	}

	j.ctx.Logger.Info().
		Str("installation_id", j.Payload.InstallationID).
		Int("repository_count", len(repositories)).
		Msg("Synced installation repositories")

	// Broadcast event
	// TODO: Broadcast RepositoriesSynced event via websocket

	return nil
}

// Failed is called when the job fails after all retries
func (j *SyncInstallationReposJob) Failed(ctx context.Context, err error) {
	j.ctx.Logger.Error().Err(err).
		Str("provider", j.Payload.Provider).
		Str("installation_id", j.Payload.InstallationID).
		Msg("Failed to sync installation repositories")
}

// NewSyncInstallationReposTask creates an asynq task for syncing installation repositories
func NewSyncInstallationReposTask(provider, installationID, teamID, userID string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeSyncInstallationRepos, SyncInstallationReposPayload{
		Provider:       provider,
		InstallationID: installationID,
		TeamID:         teamID,
		UserID:         userID,
	})
}

// NewSyncInstallationReposJob creates a new SyncInstallationReposJob
func NewSyncInstallationReposJob(ctx *JobContext, payload SyncInstallationReposPayload) *SyncInstallationReposJob {
	return &SyncInstallationReposJob{
		ctx:     ctx,
		Payload: payload,
	}
}
