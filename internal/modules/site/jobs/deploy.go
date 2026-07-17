package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/retry"
)

const (
	TypeDeploy             = "site:deploy"
	TypeDeployZeroDowntime = "site:deploy_zero_downtime"
)

// DeployPayload holds data for site deployment
type DeployPayload struct {
	SiteID       string            `json:"site_id"`
	DeploymentID string            `json:"deployment_id"`
	UserID       *string           `json:"user_id,omitempty"`
	EnvVariables map[string]string `json:"env_variables,omitempty"`
	ZeroDowntime bool              `json:"zero_downtime,omitempty"`
}

// DeployJob handles both standard and zero-downtime site deployment
type DeployJob struct {
	Deps    *JobDeps
	Payload DeployPayload

	// Model fields for Failed() callback
	site       *models.Site
	server     *servermodels.Server
	deployment *models.Deployment
}

func NewDeployJob(p DeployPayload) pkgjobs.Handler {
	return &DeployJob{Deps: deps, Payload: p}
}

func NewDeployZeroDowntimeJob(p DeployPayload) pkgjobs.Handler {
	p.ZeroDowntime = true
	return &DeployJob{Deps: deps, Payload: p}
}

// Handle executes the deploy job
func (j *DeployJob) Handle(ctx context.Context) error {
	var err error

	// Get deployment with retry (handles race condition where job runs before DB commit is visible)
	j.deployment, err = retry.WithBackoff(ctx, retry.DBRetry, func() (*models.Deployment, error) {
		return j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	})
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	// Get site
	j.site, err = j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update deployment status to installing
	j.deployment.Status = sitetypes.DeploymentStatusInstalling
	if err := j.Deps.Repos.Deployment().Update(ctx, j.deployment); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Create deployment status on git provider (if source control is configured)
	j.createProviderDeployment(ctx, j.site, j.server.ID, j.deployment)

	// Broadcast deployment started
	startMsg := "Deployment started"
	if j.Payload.ZeroDowntime {
		startMsg = "Zero-downtime deployment started"
	}
	j.broadcastDeploymentProgress(ctx, j.site.ID, j.deployment.ID, "installing", startMsg)

	// Build deploy config
	config := j.buildDeployConfig(j.site, j.deployment, j.server.TeamID)

	// Create deploy task
	task := tasks.DeploySiteTask(config)

	// Execute the deploy task on the server in background mode
	// The task's callback methods (OnSuccess, OnFailure, OnExpired) will handle completion
	// - In production mode: via HTTP webhook callbacks
	// - In local mode: via SSH streaming with direct callback invocation
	taskModel, err := j.Deps.RunTask(j.server, task).AsUser(j.site.User).TrackInDB().RunInBackground(ctx)
	if err != nil {
		j.handleDeploymentFailure(ctx, j.deployment, j.site, fmt.Sprintf("Failed to start deployment: %v", err))
		return err
	}

	// Update deployment with task ID
	j.deployment.TaskID = &taskModel.ID
	if updateErr := j.Deps.Repos.Deployment().Update(ctx, j.deployment); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Msg("Failed to update deployment with task ID")
	}

	logMsg := "Deployment task started in background"
	if j.Payload.ZeroDowntime {
		logMsg = "Zero-downtime deployment task started in background"
	}
	j.Deps.Logger.Info().
		Str("deployment_id", j.deployment.ID).
		Str("task_id", taskModel.ID).
		Msg(logMsg)

	// Job completes here - task completion is handled by callbacks
	return nil
}

// Failed handles job failure
func (j *DeployJob) Failed(ctx context.Context, err error) {
	logMsg := "Deploy job failed"
	if j.Payload.ZeroDowntime {
		logMsg = "Zero-downtime deploy job failed"
	}
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("deployment_id", j.Payload.DeploymentID).
		Msg(logMsg)

	// Update deployment status
	deployment, findErr := j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if findErr == nil {
		site, _ := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
		j.handleDeploymentFailure(ctx, deployment, site, err.Error())
	}
}

func (j *DeployJob) buildDeployConfig(site *models.Site, deployment *models.Deployment, teamID string) tasks.DeployOptions {
	repositoryURL := ""
	var hasAppAuth bool
	var tempToken, authURL, appName string

	if site.SourceControlRepositoriesID != nil {
		// Get repository and source control info
		repo, sourceControl := j.getRepositoryAndSourceControl(site)
		if repo != nil {
			repositoryURL = repo.SSHURL

			// Try to get app-based auth token
			if sourceControl != nil && j.Deps.ProviderFactory != nil {
				token, url, name := j.getAppAuthToken(context.Background(), sourceControl, repo)
				if token != "" {
					hasAppAuth = true
					tempToken = token
					authURL = url
					appName = name
					// Use HTTPS URL for app-based auth (credentials helper will provide token)
					providerType := gitproviders.GitProviderType(sourceControl.Provider)
					repositoryURL = providerType.HTTPSURL(repo.FullName)
				}
			}
		}
	}

	// Generate release timestamp for zero-downtime deployment
	var releaseTimestamp string
	if j.Payload.ZeroDowntime {
		releaseTimestamp = time.Now().Format("20060102150405")
	}

	// Generate environment variables for first deployment
	var envVars map[string]string
	if site.InstalledAt == nil {
		envVars = site.GenerateEnvironmentVariables()

		// Merge database credentials passed through the job payload
		for k, v := range j.Payload.EnvVariables {
			if envVars == nil {
				envVars = make(map[string]string)
			}
			envVars[k] = v
		}
	}

	return tasks.DeployOptions{
		Site:             site,
		Deployment:       deployment,
		TeamID:           teamID,
		RepositoryURL:    repositoryURL,
		HasAppAuth:       hasAppAuth,
		TempToken:        tempToken,
		AuthURL:          authURL,
		AppName:          appName,
		ReleaseTimestamp: releaseTimestamp,
		EnvVariables:     envVars,
	}
}

func (j *DeployJob) getRepositoryAndSourceControl(site *models.Site) (*gitmodels.SourceControlRepository, *gitmodels.SourceControl) {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return nil, nil
	}

	// Query repository from source_control_repositories table
	var repo gitmodels.SourceControlRepository
	if err := j.Deps.DB.First(&repo, "id = ?", *site.SourceControlRepositoriesID).Error; err != nil {
		return nil, nil
	}

	// Get source control
	var sourceControl *gitmodels.SourceControl
	if j.Deps.SourceControlRepo != nil {
		sc, err := j.Deps.SourceControlRepo.FindByID(context.Background(), *site.SourceControlID)
		if err == nil {
			sourceControl = sc
		}
	}

	return &repo, sourceControl
}

func (j *DeployJob) getAppAuthToken(ctx context.Context, sc *gitmodels.SourceControl, repo *gitmodels.SourceControlRepository) (token, authURL, appName string) {
	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return "", "", ""
	}

	// Build source control data for the provider
	scData := j.buildSourceControlData(sc)

	// Get provider with source control context
	provider, err := j.Deps.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sc.Provider),
		scData,
	)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to get provider for app auth")
		return "", "", ""
	}

	// Get installation token
	installToken, err := provider.GetInstallationToken(ctx, *sc.InstallationID)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to get installation token")
		return "", "", ""
	}

	// Build auth URL based on provider
	authURL = provider.GetType().AuthURL(installToken, repo.FullName)
	appName = string(sc.Provider)

	return installToken, authURL, appName
}

func (j *DeployJob) buildSourceControlData(sc *gitmodels.SourceControl) *gitproviders.SourceControlData {
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

	// Parse provider data JSON if present
	if sc.ProviderData != nil && *sc.ProviderData != "" {
		var providerData map[string]interface{}
		if err := json.Unmarshal([]byte(*sc.ProviderData), &providerData); err == nil {
			scData.ProviderData = providerData
		}
	}

	return scData
}

func (j *DeployJob) broadcastDeploymentProgress(ctx context.Context, siteID, deploymentID, status, message string) {
	site, err := j.Deps.Repos.Site().FindByID(ctx, siteID)
	if err != nil {
		return
	}

	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return
	}

	j.Deps.BroadcastServerEvent(server, "deployment.progress", map[string]interface{}{
		"team_id":       server.TeamID,
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}

func (j *DeployJob) handleDeploymentFailure(ctx context.Context, deployment *models.Deployment, site *models.Site, message string) {
	deployment.Status = sitetypes.DeploymentStatusFailed

	if err := j.Deps.Repos.Deployment().Update(ctx, deployment); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update deployment status to failed")
	}

	// If first deployment failed, mark site installation as failed
	if site != nil && site.InstalledAt == nil {
		if err := j.Deps.Repos.Site().MarkAsFailed(ctx, site.ID); err != nil {
			j.Deps.Logger.Error().Err(err).Msg("Failed to update site installation_failed_at")
		}
	}

	// Update deployment status on git provider
	j.updateProviderDeploymentStatus(ctx, site, deployment, gitproviders.DeploymentStatusFailure)

	failMsg := "Deployment failed"
	if j.Payload.ZeroDowntime {
		failMsg = "Zero-downtime deployment failed"
	}
	j.broadcastDeploymentProgress(ctx, deployment.SiteID, deployment.ID, "failed", message)
	j.Deps.Logger.Error().Str("deployment_id", deployment.ID).Str("message", message).Msg(failMsg)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployJob) processNextQueuedDeployment(ctx context.Context, siteID string) {
	// Find next queued deployment
	queuedDeployments, err := j.Deps.Repos.Deployment().FindQueuedBySite(ctx, siteID)
	if err != nil || len(queuedDeployments) == 0 {
		return
	}

	// Update the first queued deployment to pending and dispatch
	nextDeployment := &queuedDeployments[0]
	nextDeployment.Status = sitetypes.DeploymentStatusPending

	if err := j.Deps.Repos.Deployment().Update(ctx, nextDeployment); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update queued deployment status")
		return
	}

	// Get site for zero downtime check
	site, err := j.Deps.Repos.Site().FindByID(ctx, siteID)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to find site for queued deployment")
		return
	}

	// Dispatch next deployment
	if site.ZeroDowntimeDeployment {
		task, taskErr := NewDeployZeroDowntimeTask(siteID, nextDeployment.ID, "", nil)
		if taskErr != nil {
			j.Deps.Logger.Error().Err(taskErr).Msg("Failed to create deployment task for queued deployment")
			return
		}
		if err := j.Deps.DispatchTask(task); err != nil {
			j.Deps.Logger.Error().Err(err).Msg("Failed to enqueue next deployment")
		}
	} else {
		task, taskErr := NewDeployTask(siteID, nextDeployment.ID, "", nil)
		if taskErr != nil {
			j.Deps.Logger.Error().Err(taskErr).Msg("Failed to create deployment task for queued deployment")
			return
		}
		if err := j.Deps.DispatchTask(task); err != nil {
			j.Deps.Logger.Error().Err(err).Msg("Failed to enqueue next deployment")
		}
	}
}

// createProviderDeployment creates a deployment on the git provider
func (j *DeployJob) createProviderDeployment(ctx context.Context, site *models.Site, serverID string, deployment *models.Deployment) {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return
	}

	repo, sourceControl := j.getRepositoryAndSourceControl(site)
	if repo == nil || sourceControl == nil {
		return
	}

	if j.Deps.ProviderFactory == nil {
		return
	}

	scData := j.buildSourceControlData(sourceControl)
	provider, err := j.Deps.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sourceControl.Provider),
		scData,
	)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to get provider for deployment status")
		return
	}

	// Build deployment info
	gitHash := ""
	if deployment.GitHash != nil {
		gitHash = *deployment.GitHash
	}

	projectID := getProjectIDFromRepo(repo)

	description := "Deployment via Launch"
	if j.Payload.ZeroDowntime {
		description = "Zero-downtime deployment via Launch"
	}

	info := &gitproviders.DeploymentInfo{
		ServerID:     serverID,
		SiteID:       site.ID,
		DeploymentID: deployment.ID,
		RepoFullName: repo.FullName,
		Branch:       site.GetRepositoryBranch(),
		GitHash:      gitHash,
		SiteURL:      site.GetURL(),
		Environment:  "production",
		Description:  description,
		ProjectID:    projectID,
	}

	result, err := provider.CreateDeployment(ctx, info)
	if result != nil && result.Data != nil {
		// Update deployment with VCS data
		deployment.VcsData = result.Data
		if err := j.Deps.Repos.Deployment().Update(ctx, deployment); err != nil {
			j.Deps.Logger.Error().Err(err).Msg("Failed to update deployment with VCS data")
		}
	}

	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to create deployment on provider")
	}
}

// updateProviderDeploymentStatus updates the deployment status on the git provider
func (j *DeployJob) updateProviderDeploymentStatus(ctx context.Context, site *models.Site, deployment *models.Deployment, status gitproviders.DeploymentStatus) {
	if site == nil || deployment == nil {
		return
	}

	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return
	}

	if len(deployment.VcsData) == 0 {
		return
	}

	repo, sourceControl := j.getRepositoryAndSourceControl(site)
	if repo == nil || sourceControl == nil {
		return
	}

	if j.Deps.ProviderFactory == nil {
		return
	}

	scData := j.buildSourceControlData(sourceControl)
	provider, err := j.Deps.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sourceControl.Provider),
		scData,
	)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to get provider for deployment status update")
		return
	}

	projectID := getProjectIDFromRepo(repo)

	info := &gitproviders.DeploymentInfo{
		SiteID:       site.ID,
		DeploymentID: deployment.ID,
		RepoFullName: repo.FullName,
		SiteURL:      site.GetURL(),
		ProjectID:    projectID,
	}

	if err := provider.UpdateDeploymentStatus(ctx, info, deployment.VcsData, status); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update deployment status on provider")
	}
}

// getProjectIDFromRepo extracts the project ID from repository additional data
// For GitLab, the project ID is needed for deployment status API calls
func getProjectIDFromRepo(repo *gitmodels.SourceControlRepository) string {
	if repo.AdditionalData == nil || *repo.AdditionalData == "" {
		return ""
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*repo.AdditionalData), &data); err != nil {
		return ""
	}

	if id, ok := data["id"]; ok {
		return fmt.Sprintf("%v", id)
	}

	return ""
}

// NewDeployTask creates a deploy job
// Uses TaskID for deduplication to prevent the same deployment from running multiple times
func NewDeployTask(siteID, deploymentID string, userID string, envVars map[string]string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return pkgjobs.Task(TypeDeploy, DeployPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userIDPtr,
		EnvVariables: envVars,
	}, asynq.TaskID(pkgjobs.Dedup("deploy", deploymentID)))
}

// NewDeployZeroDowntimeTask creates a zero-downtime deploy job
// Uses TaskID for deduplication to prevent the same deployment from running multiple times
func NewDeployZeroDowntimeTask(siteID, deploymentID string, userID string, envVars map[string]string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return pkgjobs.Task(TypeDeployZeroDowntime, DeployPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userIDPtr,
		EnvVariables: envVars,
		ZeroDowntime: true,
	}, asynq.TaskID(pkgjobs.Dedup("deploy_zd", deploymentID)))
}
