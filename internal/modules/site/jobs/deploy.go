package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const (
	TypeDeploy             = "site:deploy"
	TypeDeployZeroDowntime = "site:deploy_zero_downtime"
)

// DeployPayload holds data for site deployment
type DeployPayload struct {
	SiteID       string  `json:"site_id"`
	DeploymentID string  `json:"deployment_id"`
	UserID       *string `json:"user_id,omitempty"`
}

// DeployJob handles standard site deployment
type DeployJob struct {
	ctx     *JobContext
	Payload DeployPayload
}

// Handle executes the deploy job
func (j *DeployJob) Handle(ctx context.Context) error {
	// Get deployment
	deployment, err := j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update deployment status to installing
	deployment.Status = enums.DeploymentStatusInstalling
	if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Create deployment status on git provider (if source control is configured)
	j.createProviderDeployment(ctx, site, server.ID, deployment)

	// Broadcast deployment started
	j.broadcastDeploymentProgress(ctx, site.ID, deployment.ID, "installing", "Deployment started")

	// Build deploy config
	config := j.buildDeployConfig(site, deployment)

	// Create deploy task
	task := tasks.DeploySite(config)

	// Execute the deploy task on the server
	result, err := j.ctx.RunTaskOnServer(server, task).AsUser(site.User).Dispatch(ctx)
	if err != nil {
		j.handleDeploymentFailure(ctx, deployment, site, fmt.Sprintf("Deployment failed: %v", err))
		return err
	}

	output := result.GetOutput()
	exitCode := result.GetExitCode()

	if exitCode != 0 {
		j.handleDeploymentFailure(ctx, deployment, site, fmt.Sprintf("Deployment failed with exit code %d: %s", exitCode, output))
		return fmt.Errorf("deployment failed with exit code %d", exitCode)
	}

	// Mark deployment as finished
	j.handleDeploymentSuccess(ctx, deployment, site, output)

	return nil
}

// Failed handles job failure
func (j *DeployJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Deploy job failed",
		"site_id", j.Payload.SiteID,
		"deployment_id", j.Payload.DeploymentID,
	)

	// Update deployment status
	deployment, findErr := j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if findErr == nil {
		site, _ := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
		j.handleDeploymentFailure(ctx, deployment, site, err.Error())
	}
}

func (j *DeployJob) buildDeployConfig(site *models.Site, deployment *models.Deployment) tasks.DeploySiteConfig {
	phpBinary := "php"
	if site.PhpVersion != nil {
		phpBinary = fmt.Sprintf("php%s", *site.PhpVersion)
	}

	repositoryURL := ""
	var hasAppAuth bool
	var tempToken, authURL, appName string

	if site.SourceControlRepositoriesID != nil {
		// Get repository and source control info
		repo, sourceControl := j.getRepositoryAndSourceControl(site)
		if repo != nil {
			repositoryURL = repo.SSHURL

			// Try to get app-based auth token
			if sourceControl != nil && j.ctx.ProviderFactory != nil {
				token, url, name := j.getAppAuthToken(context.Background(), sourceControl, repo)
				if token != "" {
					hasAppAuth = true
					tempToken = token
					authURL = url
					appName = name
				}
			}
		}
	}

	// Generate environment variables for first deployment
	var envVars map[string]string
	if site.InstalledAt == nil {
		envVars = site.GenerateEnvironmentVariables()
	}

	return tasks.DeploySiteConfig{
		SitePath:                     site.Path,
		SiteType:                     tasks.SiteType(site.Type),
		SiteAddress:                  site.Address,
		Username:                     site.User,
		PHPBinary:                    phpBinary,
		RepositoryURL:                repositoryURL,
		RepositoryBranch:             site.GetRepositoryBranch(),
		InstalledAt:                  site.InstalledAt != nil,
		ZeroDowntimeDeployment:       false,
		RepositoryDirectory:          fmt.Sprintf("%s/repository", site.Path),
		LogsDirectory:                site.GetLogsDirectory(),
		DeploymentID:                 deployment.ID,
		HasAppAuth:                   hasAppAuth,
		TempToken:                    tempToken,
		AuthURL:                      authURL,
		AppName:                      appName,
		EnvVariables:                 envVars,
		HookBeforeUpdatingRepository: stringValue(site.HookBeforeUpdatingRepository),
		HookAfterUpdatingRepository:  stringValue(site.HookAfterUpdatingRepository),
		HookBeforeMakingCurrent:      stringValue(site.HookBeforeMakingCurrent),
		HookAfterMakingCurrent:       stringValue(site.HookAfterMakingCurrent),
		SharedDirectories:            site.SharedDirectories,
		SharedFiles:                  site.SharedFiles,
		WritableDirectories:          site.WriteableDirectories,
		RetentionCount:               site.DeploymentReleasesRetention,
	}
}

func (j *DeployJob) getRepositoryAndSourceControl(site *models.Site) (*gitmodels.SourceControlRepository, *gitmodels.SourceControl) {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return nil, nil
	}

	// Query repository from source_control_repositories table
	var repo gitmodels.SourceControlRepository
	if err := j.ctx.DB.First(&repo, "id = ?", *site.SourceControlRepositoriesID).Error; err != nil {
		return nil, nil
	}

	// Get source control
	var sourceControl *gitmodels.SourceControl
	if j.ctx.SourceControlRepo != nil {
		sc, err := j.ctx.SourceControlRepo.FindByID(context.Background(), *site.SourceControlID)
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
	provider, err := j.ctx.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sc.Provider),
		scData,
	)
	if err != nil {
		j.ctx.LogError(err, "Failed to get provider for app auth")
		return "", "", ""
	}

	// Get installation token
	installToken, err := provider.GetInstallationToken(ctx, *sc.InstallationID)
	if err != nil {
		j.ctx.LogError(err, "Failed to get installation token")
		return "", "", ""
	}

	// Build auth URL based on provider
	authURL = buildAuthURL(provider.GetType(), installToken, repo.FullName)
	appName = string(sc.Provider)

	return installToken, authURL, appName
}

func (j *DeployJob) buildSourceControlData(sc *gitmodels.SourceControl) *gitproviders.SourceControlData {
	scData := &gitproviders.SourceControlData{
		ID:             sc.ID,
		UserID:         sc.UserID,
		Provider:       gitproviders.GitProviderType(sc.Provider),
		InstallationID: sc.InstallationID,
	}

	if sc.TeamID != nil {
		scData.TeamID = *sc.TeamID
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

// buildAuthURL builds the HTTPS URL with token authentication for the given provider
func buildAuthURL(providerType gitproviders.GitProviderType, token, repoFullName string) string {
	switch providerType {
	case gitproviders.GitProviderGitHub:
		return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoFullName)
	case gitproviders.GitProviderGitLab:
		return fmt.Sprintf("https://gitlab-ci-token:%s@gitlab.com/%s.git", token, repoFullName)
	case gitproviders.GitProviderBitbucket:
		return fmt.Sprintf("https://x-token-auth:%s@bitbucket.org/%s.git", token, repoFullName)
	default:
		return ""
	}
}

func (j *DeployJob) broadcastDeploymentProgress(ctx context.Context, siteID, deploymentID, status, message string) {
	site, err := j.ctx.SiteRepo.FindByID(ctx, siteID)
	if err != nil {
		return
	}

	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return
	}

	j.ctx.BroadcastServerEvent(server, "deployment.progress", map[string]interface{}{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}

func (j *DeployJob) handleDeploymentSuccess(ctx context.Context, deployment *models.Deployment, site *models.Site, output string) {
	deployment.Status = enums.DeploymentStatusFinished

	if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.ctx.LogError(err, "Failed to update deployment status to finished")
	}

	// Update deployment status on git provider
	j.updateProviderDeploymentStatus(ctx, site, deployment, gitproviders.DeploymentStatusSuccess)

	// If first deployment, dispatch InstallCaddyfile job to set up web server
	isFirstDeployment := site != nil && site.InstalledAt == nil
	if isFirstDeployment {
		// Dispatch InstallCaddyfile job - this will set installed_at on success
		task, err := NewInstallCaddyfileTask(site.ID, nil)
		if err != nil {
			j.ctx.LogError(err, "Failed to create InstallCaddyfile task")
		} else if j.ctx.Queue != nil {
			if _, err := j.ctx.Queue.Enqueue(task); err != nil {
				j.ctx.LogError(err, "Failed to enqueue InstallCaddyfile job")
			}
		}
	}

	// Restart queue workers if auto-restart is enabled
	if site != nil && site.AutoRestartQueue {
		j.restartQueueWorkers(ctx, site)
	}

	j.broadcastDeploymentProgress(ctx, deployment.SiteID, deployment.ID, "finished", "Deployment completed successfully")
	j.ctx.LogInfo("Deployment finished successfully", "deployment_id", deployment.ID)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployJob) handleDeploymentFailure(ctx context.Context, deployment *models.Deployment, site *models.Site, message string) {
	deployment.Status = enums.DeploymentStatusFailed

	if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.ctx.LogError(err, "Failed to update deployment status to failed")
	}

	// Update deployment status on git provider
	j.updateProviderDeploymentStatus(ctx, site, deployment, gitproviders.DeploymentStatusFailure)

	j.broadcastDeploymentProgress(ctx, deployment.SiteID, deployment.ID, "failed", message)
	j.ctx.LogError(nil, "Deployment failed", "deployment_id", deployment.ID, "message", message)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployJob) restartQueueWorkers(ctx context.Context, site *models.Site) {
	// Get all active queue workers for this site
	queues, err := j.ctx.QueueRepo.FindBySite(ctx, site.ID)
	if err != nil {
		j.ctx.LogError(err, "Failed to get queue workers for restart")
		return
	}

	for _, q := range queues {
		task, err := NewRestartQueueTask(site.ID, q.ID, nil)
		if err != nil {
			j.ctx.LogError(err, "Failed to create restart queue task", "queue_id", q.ID)
			continue
		}
		if j.ctx.Queue != nil {
			if _, err := j.ctx.Queue.Enqueue(task); err != nil {
				j.ctx.LogError(err, "Failed to enqueue restart queue job", "queue_id", q.ID)
			}
		}
	}
}

func (j *DeployJob) processNextQueuedDeployment(ctx context.Context, siteID string) {
	// Find next queued deployment
	queuedDeployments, err := j.ctx.DeploymentRepo.FindQueuedBySite(ctx, siteID)
	if err != nil || len(queuedDeployments) == 0 {
		return
	}

	// Update the first queued deployment to pending and dispatch
	nextDeployment := &queuedDeployments[0]
	nextDeployment.Status = enums.DeploymentStatusPending

	if err := j.ctx.DeploymentRepo.Update(ctx, nextDeployment); err != nil {
		j.ctx.LogError(err, "Failed to update queued deployment status")
		return
	}

	// Get site for zero downtime check
	site, err := j.ctx.SiteRepo.FindByID(ctx, siteID)
	if err != nil {
		j.ctx.LogError(err, "Failed to find site for queued deployment")
		return
	}

	// Dispatch next deployment
	if site.ZeroDowntimeDeployment {
		task, taskErr := NewDeployZeroDowntimeTask(siteID, nextDeployment.ID, "")
		if taskErr != nil {
			j.ctx.LogError(taskErr, "Failed to create deployment task for queued deployment")
			return
		}
		if j.ctx.Queue != nil {
			if _, err := j.ctx.Queue.Enqueue(task); err != nil {
				j.ctx.LogError(err, "Failed to enqueue next deployment")
			}
		}
	} else {
		task, taskErr := NewDeployTask(siteID, nextDeployment.ID, "")
		if taskErr != nil {
			j.ctx.LogError(taskErr, "Failed to create deployment task for queued deployment")
			return
		}
		if j.ctx.Queue != nil {
			if _, err := j.ctx.Queue.Enqueue(task); err != nil {
				j.ctx.LogError(err, "Failed to enqueue next deployment")
			}
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

	if j.ctx.ProviderFactory == nil {
		return
	}

	scData := j.buildSourceControlData(sourceControl)
	provider, err := j.ctx.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sourceControl.Provider),
		scData,
	)
	if err != nil {
		j.ctx.LogError(err, "Failed to get provider for deployment status")
		return
	}

	// Build deployment info
	gitHash := ""
	if deployment.GitHash != nil {
		gitHash = *deployment.GitHash
	}

	projectID := getProjectIDFromRepo(repo)

	info := &gitproviders.DeploymentInfo{
		ServerID:     serverID,
		SiteID:       site.ID,
		DeploymentID: deployment.ID,
		RepoFullName: repo.FullName,
		Branch:       site.GetRepositoryBranch(),
		GitHash:      gitHash,
		SiteURL:      site.GetURL(),
		Environment:  "production",
		Description:  "Deployment via Launch",
		ProjectID:    projectID,
	}

	result, err := provider.CreateDeployment(ctx, info)
	if err != nil {
		j.ctx.LogError(err, "Failed to create deployment on provider")
		return
	}

	if result != nil && result.Data != nil {
		// Update deployment with VCS data
		deployment.VcsData = result.Data
		if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
			j.ctx.LogError(err, "Failed to update deployment with VCS data")
		}
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

	if deployment.VcsData == nil || len(deployment.VcsData) == 0 {
		return
	}

	repo, sourceControl := j.getRepositoryAndSourceControl(site)
	if repo == nil || sourceControl == nil {
		return
	}

	if j.ctx.ProviderFactory == nil {
		return
	}

	scData := j.buildSourceControlData(sourceControl)
	provider, err := j.ctx.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sourceControl.Provider),
		scData,
	)
	if err != nil {
		j.ctx.LogError(err, "Failed to get provider for deployment status update")
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
		j.ctx.LogError(err, "Failed to update deployment status on provider")
	}
}

// DeployZeroDowntimeJob handles zero-downtime site deployment
type DeployZeroDowntimeJob struct {
	ctx     *JobContext
	Payload DeployPayload
}

// Handle executes the zero-downtime deploy job
func (j *DeployZeroDowntimeJob) Handle(ctx context.Context) error {
	// Get deployment
	deployment, err := j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update deployment status to installing
	deployment.Status = enums.DeploymentStatusInstalling
	if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Create deployment status on git provider (if source control is configured)
	j.createProviderDeployment(ctx, site, server.ID, deployment)

	// Broadcast deployment started
	j.broadcastDeploymentProgress(ctx, site.ID, deployment.ID, "installing", "Zero-downtime deployment started")

	// Build deploy config
	config := j.buildDeployConfig(site, deployment)

	// Create zero-downtime deploy task
	task := tasks.DeploySiteWithoutDowntime(config)

	// Execute the deploy task on the server
	result, err := j.ctx.RunTaskOnServer(server, task).AsUser(site.User).Dispatch(ctx)
	if err != nil {
		j.handleDeploymentFailure(ctx, deployment, site, fmt.Sprintf("Deployment failed: %v", err))
		return err
	}

	output := result.GetOutput()
	exitCode := result.GetExitCode()

	if exitCode != 0 {
		j.handleDeploymentFailure(ctx, deployment, site, fmt.Sprintf("Deployment failed with exit code %d: %s", exitCode, output))
		return fmt.Errorf("deployment failed with exit code %d", exitCode)
	}

	// Create release record for rollback capability
	if err := j.createRelease(ctx, site, deployment); err != nil {
		j.ctx.LogError(err, "Failed to create release record")
	}

	// Mark deployment as finished
	j.handleDeploymentSuccess(ctx, deployment, site, output)

	return nil
}

// Failed handles job failure
func (j *DeployZeroDowntimeJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Zero-downtime deploy job failed",
		"site_id", j.Payload.SiteID,
		"deployment_id", j.Payload.DeploymentID,
	)

	// Update deployment status
	deployment, findErr := j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if findErr == nil {
		site, _ := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
		j.handleDeploymentFailure(ctx, deployment, site, err.Error())
	}
}

func (j *DeployZeroDowntimeJob) buildDeployConfig(site *models.Site, deployment *models.Deployment) tasks.DeploySiteConfig {
	phpBinary := "php"
	if site.PhpVersion != nil {
		phpBinary = fmt.Sprintf("php%s", *site.PhpVersion)
	}

	repositoryURL := ""
	var hasAppAuth bool
	var tempToken, authURL, appName string

	if site.SourceControlRepositoriesID != nil {
		// Get repository and source control info
		repo, sourceControl := j.getRepositoryAndSourceControl(site)
		if repo != nil {
			repositoryURL = repo.SSHURL

			// Try to get app-based auth token
			if sourceControl != nil && j.ctx.ProviderFactory != nil {
				token, url, name := j.getAppAuthToken(context.Background(), sourceControl, repo)
				if token != "" {
					hasAppAuth = true
					tempToken = token
					authURL = url
					appName = name
				}
			}
		}
	}

	// Generate release directory with timestamp
	releaseTimestamp := time.Now().Format("20060102150405")
	releaseDirectory := fmt.Sprintf("%s/releases/%s", site.Path, releaseTimestamp)

	// Generate environment variables for first deployment
	var envVars map[string]string
	if site.InstalledAt == nil {
		envVars = site.GenerateEnvironmentVariables()
	}

	return tasks.DeploySiteConfig{
		SitePath:                     site.Path,
		SiteType:                     tasks.SiteType(site.Type),
		SiteAddress:                  site.Address,
		Username:                     site.User,
		PHPBinary:                    phpBinary,
		RepositoryURL:                repositoryURL,
		RepositoryBranch:             site.GetRepositoryBranch(),
		InstalledAt:                  site.InstalledAt != nil,
		ZeroDowntimeDeployment:       true,
		RepositoryDirectory:          fmt.Sprintf("%s/repository", site.Path),
		LogsDirectory:                site.GetLogsDirectory(),
		SharedDirectory:              fmt.Sprintf("%s/shared", site.Path),
		ReleaseDirectory:             releaseDirectory,
		ReleasesDirectory:            fmt.Sprintf("%s/releases", site.Path),
		CurrentDirectory:             fmt.Sprintf("%s/current", site.Path),
		DeploymentID:                 deployment.ID,
		HasAppAuth:                   hasAppAuth,
		TempToken:                    tempToken,
		AuthURL:                      authURL,
		AppName:                      appName,
		EnvVariables:                 envVars,
		HookBeforeUpdatingRepository: stringValue(site.HookBeforeUpdatingRepository),
		HookAfterUpdatingRepository:  stringValue(site.HookAfterUpdatingRepository),
		HookBeforeMakingCurrent:      stringValue(site.HookBeforeMakingCurrent),
		HookAfterMakingCurrent:       stringValue(site.HookAfterMakingCurrent),
		SharedDirectories:            site.SharedDirectories,
		SharedFiles:                  site.SharedFiles,
		WritableDirectories:          site.WriteableDirectories,
		LatestDeploymentTimestamp:    releaseTimestamp,
		RetentionCount:               site.DeploymentReleasesRetention,
	}
}

func (j *DeployZeroDowntimeJob) getRepositoryAndSourceControl(site *models.Site) (*gitmodels.SourceControlRepository, *gitmodels.SourceControl) {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return nil, nil
	}

	// Query repository from source_control_repositories table
	var repo gitmodels.SourceControlRepository
	if err := j.ctx.DB.First(&repo, "id = ?", *site.SourceControlRepositoriesID).Error; err != nil {
		return nil, nil
	}

	// Get source control
	var sourceControl *gitmodels.SourceControl
	if j.ctx.SourceControlRepo != nil {
		sc, err := j.ctx.SourceControlRepo.FindByID(context.Background(), *site.SourceControlID)
		if err == nil {
			sourceControl = sc
		}
	}

	return &repo, sourceControl
}

func (j *DeployZeroDowntimeJob) getAppAuthToken(ctx context.Context, sc *gitmodels.SourceControl, repo *gitmodels.SourceControlRepository) (token, authURL, appName string) {
	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return "", "", ""
	}

	// Build source control data for the provider
	scData := j.buildSourceControlData(sc)

	// Get provider with source control context
	provider, err := j.ctx.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sc.Provider),
		scData,
	)
	if err != nil {
		j.ctx.LogError(err, "Failed to get provider for app auth")
		return "", "", ""
	}

	// Get installation token
	installToken, err := provider.GetInstallationToken(ctx, *sc.InstallationID)
	if err != nil {
		j.ctx.LogError(err, "Failed to get installation token")
		return "", "", ""
	}

	// Build auth URL based on provider
	authURL = buildAuthURL(provider.GetType(), installToken, repo.FullName)
	appName = string(sc.Provider)

	return installToken, authURL, appName
}

func (j *DeployZeroDowntimeJob) buildSourceControlData(sc *gitmodels.SourceControl) *gitproviders.SourceControlData {
	scData := &gitproviders.SourceControlData{
		ID:             sc.ID,
		UserID:         sc.UserID,
		Provider:       gitproviders.GitProviderType(sc.Provider),
		InstallationID: sc.InstallationID,
	}

	if sc.TeamID != nil {
		scData.TeamID = *sc.TeamID
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

func (j *DeployZeroDowntimeJob) broadcastDeploymentProgress(ctx context.Context, siteID, deploymentID, status, message string) {
	site, err := j.ctx.SiteRepo.FindByID(ctx, siteID)
	if err != nil {
		return
	}

	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return
	}

	j.ctx.BroadcastServerEvent(server, "deployment.progress", map[string]interface{}{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}

func (j *DeployZeroDowntimeJob) handleDeploymentSuccess(ctx context.Context, deployment *models.Deployment, site *models.Site, output string) {
	deployment.Status = enums.DeploymentStatusFinished

	if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.ctx.LogError(err, "Failed to update deployment status to finished")
	}

	// Update deployment status on git provider
	j.updateProviderDeploymentStatus(ctx, site, deployment, gitproviders.DeploymentStatusSuccess)

	// If first deployment, dispatch InstallCaddyfile job to set up web server
	isFirstDeployment := site != nil && site.InstalledAt == nil
	if isFirstDeployment {
		// Dispatch InstallCaddyfile job - this will set installed_at on success
		task, err := NewInstallCaddyfileTask(site.ID, nil)
		if err != nil {
			j.ctx.LogError(err, "Failed to create InstallCaddyfile task")
		} else if j.ctx.Queue != nil {
			if _, err := j.ctx.Queue.Enqueue(task); err != nil {
				j.ctx.LogError(err, "Failed to enqueue InstallCaddyfile job")
			}
		}
	}

	// Restart queue workers if auto-restart is enabled
	if site != nil && site.AutoRestartQueue {
		j.restartQueueWorkers(ctx, site)
	}

	j.broadcastDeploymentProgress(ctx, deployment.SiteID, deployment.ID, "finished", "Deployment completed successfully")
	j.ctx.LogInfo("Zero-downtime deployment finished successfully", "deployment_id", deployment.ID)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployZeroDowntimeJob) handleDeploymentFailure(ctx context.Context, deployment *models.Deployment, site *models.Site, message string) {
	deployment.Status = enums.DeploymentStatusFailed

	if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.ctx.LogError(err, "Failed to update deployment status to failed")
	}

	// Update deployment status on git provider
	j.updateProviderDeploymentStatus(ctx, site, deployment, gitproviders.DeploymentStatusFailure)

	j.broadcastDeploymentProgress(ctx, deployment.SiteID, deployment.ID, "failed", message)
	j.ctx.LogError(nil, "Zero-downtime deployment failed", "deployment_id", deployment.ID, "message", message)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployZeroDowntimeJob) restartQueueWorkers(ctx context.Context, site *models.Site) {
	// Get all active queue workers for this site
	queues, err := j.ctx.QueueRepo.FindBySite(ctx, site.ID)
	if err != nil {
		j.ctx.LogError(err, "Failed to get queue workers for restart")
		return
	}

	for _, q := range queues {
		task, err := NewRestartQueueTask(site.ID, q.ID, nil)
		if err != nil {
			j.ctx.LogError(err, "Failed to create restart queue task", "queue_id", q.ID)
			continue
		}
		if j.ctx.Queue != nil {
			if _, err := j.ctx.Queue.Enqueue(task); err != nil {
				j.ctx.LogError(err, "Failed to enqueue restart queue job", "queue_id", q.ID)
			}
		}
	}
}

func (j *DeployZeroDowntimeJob) processNextQueuedDeployment(ctx context.Context, siteID string) {
	queuedDeployments, err := j.ctx.DeploymentRepo.FindQueuedBySite(ctx, siteID)
	if err != nil || len(queuedDeployments) == 0 {
		return
	}

	nextDeployment := &queuedDeployments[0]
	nextDeployment.Status = enums.DeploymentStatusPending

	if err := j.ctx.DeploymentRepo.Update(ctx, nextDeployment); err != nil {
		j.ctx.LogError(err, "Failed to update queued deployment status")
		return
	}

	site, err := j.ctx.SiteRepo.FindByID(ctx, siteID)
	if err != nil {
		j.ctx.LogError(err, "Failed to find site for queued deployment")
		return
	}

	if site.ZeroDowntimeDeployment {
		task, taskErr := NewDeployZeroDowntimeTask(siteID, nextDeployment.ID, "")
		if taskErr != nil {
			j.ctx.LogError(taskErr, "Failed to create deployment task for queued deployment")
			return
		}
		if j.ctx.Queue != nil {
			if _, err := j.ctx.Queue.Enqueue(task); err != nil {
				j.ctx.LogError(err, "Failed to enqueue next deployment")
			}
		}
	} else {
		task, taskErr := NewDeployTask(siteID, nextDeployment.ID, "")
		if taskErr != nil {
			j.ctx.LogError(taskErr, "Failed to create deployment task for queued deployment")
			return
		}
		if j.ctx.Queue != nil {
			if _, err := j.ctx.Queue.Enqueue(task); err != nil {
				j.ctx.LogError(err, "Failed to enqueue next deployment")
			}
		}
	}
}

func (j *DeployZeroDowntimeJob) createRelease(ctx context.Context, site *models.Site, deployment *models.Deployment) error {
	release := &models.Release{
		SiteID:       site.ID,
		DeploymentID: deployment.ID,
		Path:         fmt.Sprintf("%s/releases/%s", site.Path, time.Now().Format("20060102150405")),
	}

	return j.ctx.ReleaseRepo.Create(ctx, release)
}

// createProviderDeployment creates a deployment on the git provider
func (j *DeployZeroDowntimeJob) createProviderDeployment(ctx context.Context, site *models.Site, serverID string, deployment *models.Deployment) {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return
	}

	repo, sourceControl := j.getRepositoryAndSourceControl(site)
	if repo == nil || sourceControl == nil {
		return
	}

	if j.ctx.ProviderFactory == nil {
		return
	}

	scData := j.buildSourceControlData(sourceControl)
	provider, err := j.ctx.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sourceControl.Provider),
		scData,
	)
	if err != nil {
		j.ctx.LogError(err, "Failed to get provider for deployment status")
		return
	}

	gitHash := ""
	if deployment.GitHash != nil {
		gitHash = *deployment.GitHash
	}

	projectID := getProjectIDFromRepo(repo)

	info := &gitproviders.DeploymentInfo{
		ServerID:     serverID,
		SiteID:       site.ID,
		DeploymentID: deployment.ID,
		RepoFullName: repo.FullName,
		Branch:       site.GetRepositoryBranch(),
		GitHash:      gitHash,
		SiteURL:      site.GetURL(),
		Environment:  "production",
		Description:  "Zero-downtime deployment via Launch",
		ProjectID:    projectID,
	}

	result, err := provider.CreateDeployment(ctx, info)
	if err != nil {
		j.ctx.LogError(err, "Failed to create deployment on provider")
		return
	}

	if result != nil && result.Data != nil {
		deployment.VcsData = result.Data
		if err := j.ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
			j.ctx.LogError(err, "Failed to update deployment with VCS data")
		}
	}
}

// updateProviderDeploymentStatus updates the deployment status on the git provider
func (j *DeployZeroDowntimeJob) updateProviderDeploymentStatus(ctx context.Context, site *models.Site, deployment *models.Deployment, status gitproviders.DeploymentStatus) {
	if site == nil || deployment == nil {
		return
	}

	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return
	}

	if deployment.VcsData == nil || len(deployment.VcsData) == 0 {
		return
	}

	repo, sourceControl := j.getRepositoryAndSourceControl(site)
	if repo == nil || sourceControl == nil {
		return
	}

	if j.ctx.ProviderFactory == nil {
		return
	}

	scData := j.buildSourceControlData(sourceControl)
	provider, err := j.ctx.ProviderFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sourceControl.Provider),
		scData,
	)
	if err != nil {
		j.ctx.LogError(err, "Failed to get provider for deployment status update")
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
		j.ctx.LogError(err, "Failed to update deployment status on provider")
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

// Helper function
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// NewDeployJob creates a new DeployJob with the given context and payload
func NewDeployJob(ctx *JobContext, payload DeployPayload) *DeployJob {
	return &DeployJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewDeployZeroDowntimeJob creates a new DeployZeroDowntimeJob with the given context and payload
func NewDeployZeroDowntimeJob(ctx *JobContext, payload DeployPayload) *DeployZeroDowntimeJob {
	return &DeployZeroDowntimeJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewDeployTask creates a deploy job
func NewDeployTask(siteID, deploymentID string, userID string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return pkgjobs.NewTask(TypeDeploy, DeployPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userIDPtr,
	})
}

// NewDeployZeroDowntimeTask creates a zero-downtime deploy job
func NewDeployZeroDowntimeTask(siteID, deploymentID string, userID string) (*asynq.Task, error) {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}
	return pkgjobs.NewTask(TypeDeployZeroDowntime, DeployPayload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       userIDPtr,
	})
}
