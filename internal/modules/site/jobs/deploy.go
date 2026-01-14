package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
)

// DeployJob handles standard site deployment
type DeployJob struct {
	SiteJobBase
	Payload DeployPayload
}

// Type returns the job type
func (j *DeployJob) Type() string {
	return TypeDeploy
}

// Handle executes the deploy job
func (j *DeployJob) Handle(ctx context.Context) error {
	// Get deployment
	deployment, err := j.Ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepo.FindServerByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update deployment status to installing
	deployment.Status = enums.DeploymentStatusInstalling
	now := time.Now()
	deployment.StartedAt = &now
	if err := j.Ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Broadcast deployment started
	j.broadcastDeploymentProgress(site.ID, deployment.ID, "installing", "Deployment started")

	// Build deploy config
	config := j.buildDeployConfig(site, deployment)

	// Create deploy task
	task := tasks.DeploySite(config)

	// Execute the deploy task on the server
	result, err := j.RunTaskOnServer(server, task).AsUser(site.User).Dispatch(ctx)
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
	j.LogError(err, "Deploy job failed",
		"site_id", j.Payload.SiteID,
		"deployment_id", j.Payload.DeploymentID,
	)

	// Update deployment status
	deployment, findErr := j.Ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if findErr == nil {
		site, _ := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
		j.handleDeploymentFailure(ctx, deployment, site, err.Error())
	}
}

func (j *DeployJob) buildDeployConfig(site *models.Site, deployment *models.Deployment) tasks.DeploySiteConfig {
	phpBinary := "php"
	if site.PhpVersion != nil {
		phpBinary = fmt.Sprintf("php%s", *site.PhpVersion)
	}

	repositoryURL := ""
	if site.SourceControlRepositoriesID != nil {
		// Build repository URL from source control
		repositoryURL = j.getRepositoryURL(site)
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

func (j *DeployJob) getRepositoryURL(site *models.Site) string {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return ""
	}

	// Query repository URL from source_control_repositories table
	var repo struct {
		CloneURL string `gorm:"column:clone_url"`
	}

	err := j.DB.Table("source_control_repositories").
		Select("clone_url").
		Where("id = ?", *site.SourceControlRepositoriesID).
		First(&repo).Error

	if err != nil {
		return ""
	}

	return repo.CloneURL
}

func (j *DeployJob) broadcastDeploymentProgress(siteID, deploymentID, status, message string) {
	j.BroadcastSiteEvent(siteID, "deployment.progress", map[string]interface{}{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}

func (j *DeployJob) handleDeploymentSuccess(ctx context.Context, deployment *models.Deployment, site *models.Site, output string) {
	now := time.Now()
	deployment.Status = enums.DeploymentStatusFinished
	deployment.FinishedAt = &now
	deployment.Output = &output

	if err := j.Ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.LogError(err, "Failed to update deployment status to finished")
	}

	// If first deployment, dispatch InstallCaddyfile job to set up web server
	isFirstDeployment := site != nil && site.InstalledAt == nil
	if isFirstDeployment {
		// Dispatch InstallCaddyfile job - this will set installed_at on success
		task, err := NewInstallCaddyfileTask(site.ID, nil)
		if err != nil {
			j.LogError(err, "Failed to create InstallCaddyfile task")
		} else if j.Ctx.Queue != nil {
			if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
				j.LogError(err, "Failed to enqueue InstallCaddyfile job")
			}
		}
	}

	// Restart queue workers if auto-restart is enabled
	if site != nil && site.AutoRestartQueue {
		j.restartQueueWorkers(ctx, site)
	}

	j.broadcastDeploymentProgress(deployment.SiteID, deployment.ID, "finished", "Deployment completed successfully")
	j.LogInfo("Deployment finished successfully", "deployment_id", deployment.ID)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployJob) handleDeploymentFailure(ctx context.Context, deployment *models.Deployment, site *models.Site, message string) {
	now := time.Now()
	deployment.Status = enums.DeploymentStatusFailed
	deployment.FinishedAt = &now
	deployment.Output = &message

	if err := j.Ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.LogError(err, "Failed to update deployment status to failed")
	}

	j.broadcastDeploymentProgress(deployment.SiteID, deployment.ID, "failed", message)
	j.LogError(nil, "Deployment failed", "deployment_id", deployment.ID, "message", message)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployJob) restartQueueWorkers(ctx context.Context, site *models.Site) {
	// Get all active queue workers for this site
	queues, err := j.Ctx.QueueRepo.FindBySite(ctx, site.ID)
	if err != nil {
		j.LogError(err, "Failed to get queue workers for restart")
		return
	}

	for _, q := range queues {
		task, err := NewRestartQueueTask(site.ID, q.ID, nil)
		if err != nil {
			j.LogError(err, "Failed to create restart queue task", "queue_id", q.ID)
			continue
		}
		if j.Ctx.Queue != nil {
			if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
				j.LogError(err, "Failed to enqueue restart queue job", "queue_id", q.ID)
			}
		}
	}
}

func (j *DeployJob) processNextQueuedDeployment(ctx context.Context, siteID string) {
	// Find next queued deployment
	queuedDeployments, err := j.Ctx.DeploymentRepo.FindQueuedBySite(ctx, siteID)
	if err != nil || len(queuedDeployments) == 0 {
		return
	}

	// Update the first queued deployment to pending and dispatch
	nextDeployment := &queuedDeployments[0]
	nextDeployment.Status = enums.DeploymentStatusPending

	if err := j.Ctx.DeploymentRepo.Update(ctx, nextDeployment); err != nil {
		j.LogError(err, "Failed to update queued deployment status")
		return
	}

	// Get site for zero downtime check
	site, err := j.Ctx.SiteRepo.FindByID(ctx, siteID)
	if err != nil {
		j.LogError(err, "Failed to find site for queued deployment")
		return
	}

	// Dispatch next deployment
	if site.ZeroDowntimeDeployment {
		task, taskErr := NewDeployZeroDowntimeTask(siteID, nextDeployment.ID, "")
		if taskErr != nil {
			j.LogError(taskErr, "Failed to create deployment task for queued deployment")
			return
		}
		if j.Ctx.Queue != nil {
			if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
				j.LogError(err, "Failed to enqueue next deployment")
			}
		}
	} else {
		task, taskErr := NewDeployTask(siteID, nextDeployment.ID, "")
		if taskErr != nil {
			j.LogError(taskErr, "Failed to create deployment task for queued deployment")
			return
		}
		if j.Ctx.Queue != nil {
			if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
				j.LogError(err, "Failed to enqueue next deployment")
			}
		}
	}
}

// DeployZeroDowntimeJob handles zero-downtime site deployment
type DeployZeroDowntimeJob struct {
	SiteJobBase
	Payload DeployPayload
}

// Type returns the job type
func (j *DeployZeroDowntimeJob) Type() string {
	return TypeDeployZeroDowntime
}

// Handle executes the zero-downtime deploy job
func (j *DeployZeroDowntimeJob) Handle(ctx context.Context) error {
	// Get deployment
	deployment, err := j.Ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepo.FindServerByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update deployment status to installing
	deployment.Status = enums.DeploymentStatusInstalling
	now := time.Now()
	deployment.StartedAt = &now
	if err := j.Ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Broadcast deployment started
	j.broadcastDeploymentProgress(site.ID, deployment.ID, "installing", "Zero-downtime deployment started")

	// Build deploy config
	config := j.buildDeployConfig(site, deployment)

	// Create zero-downtime deploy task
	task := tasks.DeploySiteWithoutDowntime(config)

	// Execute the deploy task on the server
	result, err := j.RunTaskOnServer(server, task).AsUser(site.User).Dispatch(ctx)
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
		j.LogError(err, "Failed to create release record")
	}

	// Mark deployment as finished
	j.handleDeploymentSuccess(ctx, deployment, site, output)

	return nil
}

// Failed handles job failure
func (j *DeployZeroDowntimeJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Zero-downtime deploy job failed",
		"site_id", j.Payload.SiteID,
		"deployment_id", j.Payload.DeploymentID,
	)

	// Update deployment status
	deployment, findErr := j.Ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
	if findErr == nil {
		site, _ := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
		j.handleDeploymentFailure(ctx, deployment, site, err.Error())
	}
}

func (j *DeployZeroDowntimeJob) buildDeployConfig(site *models.Site, deployment *models.Deployment) tasks.DeploySiteConfig {
	phpBinary := "php"
	if site.PhpVersion != nil {
		phpBinary = fmt.Sprintf("php%s", *site.PhpVersion)
	}

	repositoryURL := ""
	if site.SourceControlRepositoriesID != nil {
		repositoryURL = j.getRepositoryURL(site)
	}

	// Generate release directory with timestamp
	releaseTimestamp := time.Now().Format("20060102150405")
	releaseDirectory := fmt.Sprintf("%s/releases/%s", site.Path, releaseTimestamp)

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

func (j *DeployZeroDowntimeJob) getRepositoryURL(site *models.Site) string {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return ""
	}

	var repo struct {
		CloneURL string `gorm:"column:clone_url"`
	}

	err := j.DB.Table("source_control_repositories").
		Select("clone_url").
		Where("id = ?", *site.SourceControlRepositoriesID).
		First(&repo).Error

	if err != nil {
		return ""
	}

	return repo.CloneURL
}

func (j *DeployZeroDowntimeJob) broadcastDeploymentProgress(siteID, deploymentID, status, message string) {
	j.BroadcastSiteEvent(siteID, "deployment.progress", map[string]interface{}{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}

func (j *DeployZeroDowntimeJob) handleDeploymentSuccess(ctx context.Context, deployment *models.Deployment, site *models.Site, output string) {
	now := time.Now()
	deployment.Status = enums.DeploymentStatusFinished
	deployment.FinishedAt = &now
	deployment.Output = &output

	if err := j.Ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.LogError(err, "Failed to update deployment status to finished")
	}

	// If first deployment, dispatch InstallCaddyfile job to set up web server
	isFirstDeployment := site != nil && site.InstalledAt == nil
	if isFirstDeployment {
		// Dispatch InstallCaddyfile job - this will set installed_at on success
		task, err := NewInstallCaddyfileTask(site.ID, nil)
		if err != nil {
			j.LogError(err, "Failed to create InstallCaddyfile task")
		} else if j.Ctx.Queue != nil {
			if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
				j.LogError(err, "Failed to enqueue InstallCaddyfile job")
			}
		}
	}

	// Restart queue workers if auto-restart is enabled
	if site != nil && site.AutoRestartQueue {
		j.restartQueueWorkers(ctx, site)
	}

	j.broadcastDeploymentProgress(deployment.SiteID, deployment.ID, "finished", "Deployment completed successfully")
	j.LogInfo("Zero-downtime deployment finished successfully", "deployment_id", deployment.ID)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployZeroDowntimeJob) handleDeploymentFailure(ctx context.Context, deployment *models.Deployment, site *models.Site, message string) {
	now := time.Now()
	deployment.Status = enums.DeploymentStatusFailed
	deployment.FinishedAt = &now
	deployment.Output = &message

	if err := j.Ctx.DeploymentRepo.Update(ctx, deployment); err != nil {
		j.LogError(err, "Failed to update deployment status to failed")
	}

	j.broadcastDeploymentProgress(deployment.SiteID, deployment.ID, "failed", message)
	j.LogError(nil, "Zero-downtime deployment failed", "deployment_id", deployment.ID, "message", message)

	// Process next queued deployment if any
	j.processNextQueuedDeployment(ctx, deployment.SiteID)
}

func (j *DeployZeroDowntimeJob) restartQueueWorkers(ctx context.Context, site *models.Site) {
	// Get all active queue workers for this site
	queues, err := j.Ctx.QueueRepo.FindBySite(ctx, site.ID)
	if err != nil {
		j.LogError(err, "Failed to get queue workers for restart")
		return
	}

	for _, q := range queues {
		task, err := NewRestartQueueTask(site.ID, q.ID, nil)
		if err != nil {
			j.LogError(err, "Failed to create restart queue task", "queue_id", q.ID)
			continue
		}
		if j.Ctx.Queue != nil {
			if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
				j.LogError(err, "Failed to enqueue restart queue job", "queue_id", q.ID)
			}
		}
	}
}

func (j *DeployZeroDowntimeJob) processNextQueuedDeployment(ctx context.Context, siteID string) {
	queuedDeployments, err := j.Ctx.DeploymentRepo.FindQueuedBySite(ctx, siteID)
	if err != nil || len(queuedDeployments) == 0 {
		return
	}

	nextDeployment := &queuedDeployments[0]
	nextDeployment.Status = enums.DeploymentStatusPending

	if err := j.Ctx.DeploymentRepo.Update(ctx, nextDeployment); err != nil {
		j.LogError(err, "Failed to update queued deployment status")
		return
	}

	site, err := j.Ctx.SiteRepo.FindByID(ctx, siteID)
	if err != nil {
		j.LogError(err, "Failed to find site for queued deployment")
		return
	}

	if site.ZeroDowntimeDeployment {
		task, taskErr := NewDeployZeroDowntimeTask(siteID, nextDeployment.ID, "")
		if taskErr != nil {
			j.LogError(taskErr, "Failed to create deployment task for queued deployment")
			return
		}
		if j.Ctx.Queue != nil {
			if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
				j.LogError(err, "Failed to enqueue next deployment")
			}
		}
	} else {
		task, taskErr := NewDeployTask(siteID, nextDeployment.ID, "")
		if taskErr != nil {
			j.LogError(taskErr, "Failed to create deployment task for queued deployment")
			return
		}
		if j.Ctx.Queue != nil {
			if _, err := j.Ctx.Queue.Enqueue(task); err != nil {
				j.LogError(err, "Failed to enqueue next deployment")
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

	return j.Ctx.ReleaseRepo.Create(ctx, release)
}

// Helper function
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
