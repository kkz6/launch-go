package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUpdateProviderDeploymentStatus = "site:update_provider_deployment_status"

// UpdateProviderDeploymentStatusPayload holds data for updating deployment status on git provider
type UpdateProviderDeploymentStatusPayload struct {
	SiteID       string `json:"site_id"`
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"` // "success" or "failure"
}

// UpdateProviderDeploymentStatusJob updates deployment status on the git provider
type UpdateProviderDeploymentStatusJob struct {
	Deps    *JobDeps
	Payload UpdateProviderDeploymentStatusPayload
}

func NewUpdateProviderDeploymentStatusJob(p UpdateProviderDeploymentStatusPayload) pkgjobs.Handler {
	return &UpdateProviderDeploymentStatusJob{Deps: deps, Payload: p}
}

func (j *UpdateProviderDeploymentStatusJob) Handle(ctx context.Context) error {
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return nil
	}

	deployment, err := j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	if len(deployment.VcsData) == 0 {
		return nil
	}

	// Get repository
	var repo gitmodels.SourceControlRepository
	if err := j.Deps.DB.First(&repo, "id = ?", *site.SourceControlRepositoriesID).Error; err != nil {
		return nil
	}

	// Get source control
	if j.Deps.SourceControlRepo == nil || j.Deps.ProviderFactory == nil {
		return nil
	}

	sourceControl, err := j.Deps.SourceControlRepo.FindByID(ctx, *site.SourceControlID)
	if err != nil {
		return nil
	}

	scData := buildSourceControlData(sourceControl)
	provider, err := j.Deps.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sourceControl.Provider),
		scData,
	)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	status := gitproviders.DeploymentStatus(j.Payload.Status)

	projectID := getProjectIDFromRepo(&repo)
	info := &gitproviders.DeploymentInfo{
		SiteID:       site.ID,
		DeploymentID: deployment.ID,
		RepoFullName: repo.FullName,
		SiteURL:      site.GetURL(),
		ProjectID:    projectID,
	}

	if err := provider.UpdateDeploymentStatus(ctx, info, deployment.VcsData, status); err != nil {
		return fmt.Errorf("failed to update deployment status on provider: %w", err)
	}

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("deployment_id", deployment.ID).
		Str("status", j.Payload.Status).
		Msg("Updated deployment status on git provider")

	return nil
}

func (j *UpdateProviderDeploymentStatusJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("deployment_id", j.Payload.DeploymentID).
		Msg("Failed to update provider deployment status")
}

func buildSourceControlData(sc *gitmodels.SourceControl) *gitproviders.SourceControlData {
	scData := &gitproviders.SourceControlData{
		ID:             sc.ID,
		UserID:         sc.UserID,
		TeamID:         sc.TeamID,
		Provider:       gitproviders.GitProviderType(sc.Provider),
		InstallationID: sc.InstallationID,
	}

	if sc.URL != nil {
		scData.URL = sc.URL
	}
	if sc.Login != nil {
		scData.Login = sc.Login
	}
	if sc.Name != nil {
		scData.Name = sc.Name
	}
	if sc.Type != nil {
		scData.Type = sc.Type
	}

	if sc.ProviderData != nil && *sc.ProviderData != "" {
		var providerData map[string]any
		if err := json.Unmarshal([]byte(*sc.ProviderData), &providerData); err == nil {
			scData.ProviderData = providerData
		}
	}

	return scData
}
