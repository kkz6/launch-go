package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	gitaccess "github.com/kkz6/launch-go/internal/modules/git/access"
	siteaccess "github.com/kkz6/launch-go/internal/modules/site/gitaccess"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/deployments"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/retry"
)

const (
	TypeDeploy             = deployments.TypeDeploy
	TypeDeployZeroDowntime = deployments.TypeDeployZeroDowntime
)

// DeployPayload holds data for site deployment
type DeployPayload = deployments.Payload

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

	if _, err := siteaccess.ForDeployment(ctx, j.Deps.DB, j.Deps.ProviderFactory, j.site, j.deployment, gitaccess.Clone); err != nil {
		return err
	}

	// Get server
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	if j.site.TeamID != j.server.TeamID {
		return fmt.Errorf("site does not belong to server account")
	}

	// Save the exact release identity before starting any remote work. New names
	// are deterministic per deployment, including concurrent queue redeliveries.
	if j.Payload.ZeroDowntime {
		if j.deployment.ReleaseName == nil && j.deployment.TaskID == nil {
			if j.deployment.CreatedAt == nil {
				return fmt.Errorf("cannot name deployment release without creation time")
			}
			name := j.deployment.CreatedAt.UTC().Format("20060102150405") + "-" + j.deployment.ID
			j.deployment.ReleaseName = &name
		}
		if _, err := resolveDeploymentRelease(ctx, j.Deps.DB, j.deployment, j.server.ID); err != nil {
			return fmt.Errorf("resolve deployment release: %w", err)
		}
	}

	// Update deployment status to installing, persisting the release name too.
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
	config, err := j.buildDeployConfig(ctx, j.site, j.deployment, j.server.TeamID)
	if err != nil {
		return err
	}
	// Linking the task is required before SSH starts so failed preflight and
	// installation attempts keep their log target. Preserve repository checks.
	var taskLinkErr error
	beforeExecution := config.BeforeExecution
	config.BeforeExecution = func(ctx context.Context) error {
		if taskLinkErr != nil {
			// The background task owns this failure and its queue advancement.
			// Returning it from Handle too would invoke both failure callbacks.
			return taskLinkErr
		}
		if beforeExecution != nil {
			return beforeExecution(ctx)
		}
		return nil
	}
	task := tasks.DeploySiteTask(config)

	// Execute the deploy task on the server in background mode
	// The task's callback methods (OnSuccess, OnFailure, OnExpired) will handle completion
	// - In production mode: via HTTP webhook callbacks
	// - In local mode: via SSH streaming with direct callback invocation
	taskModel, err := j.Deps.RunTask(j.server, task).AsUser(j.site.User).ForSite(j.site.ID).TrackInDB().
		OnTaskCreated(func(taskID string) {
			result := j.Deps.DB.WithContext(ctx).Model(&models.Deployment{}).
				Where("id = ? AND site_id = ? AND team_id = ?", j.deployment.ID, j.site.ID, j.site.TeamID).
				Update("task_id", taskID)
			if result.Error != nil {
				taskLinkErr = fmt.Errorf("persist deployment task: %w", result.Error)
			} else if result.RowsAffected != 1 {
				taskLinkErr = fmt.Errorf("deployment scope changed before linking task")
			} else {
				j.deployment.TaskID = &taskID
			}
		}).RunInBackground(ctx)
	if err != nil {
		// Failed handles the terminal state and queue advancement once. Include
		// linkage failures when connection preflight stopped before its guard ran.
		return fmt.Errorf("failed to start deployment: %w", errors.Join(taskLinkErr, err))
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

	j.handleDeploymentFailure(ctx, deploymentPreparationFailureMessage(err))
}

// Only known preparation errors may appear in saved diagnostics. Upstream errors
// can contain credentials or environment values, so never persist their raw text.
func deploymentPreparationFailureMessage(err error) string {
	if errors.Is(err, gitaccess.ErrAccessDenied) {
		return "Repository access is unavailable. Reconnect GitHub and select this site's repository for this account, then start a new deployment."
	}
	if errors.Is(err, gitaccess.ErrStaleAccess) {
		return "Repository access changed after this deployment was requested. Confirm the selected repository, then start a new deployment."
	}
	if err != nil {
		switch err.Error() {
		case "select a connection and a repository together", "selected repository does not belong to this connection":
			return "The repository selection is incomplete or does not match its connection. Select the connection and repository in site settings, then start a new deployment."
		case "selected source connection is unavailable", "repository connection is unavailable", "cannot obtain selected repository credentials":
			return "The selected repository connection is unavailable. Check or reconnect it and select this site's repository, then start a new deployment."
		case "repository authorization is missing; start a new deployment", "repository authorization is invalid; start a new deployment", "legacy deployment has no repository authorization; start a new deployment":
			return "This deployment has no valid saved repository authorization. Confirm the repository selection and start a new deployment."
		case "dependency repository is unavailable", "cannot obtain selected dependency credentials":
			return "A selected dependency repository is unavailable. Check its connection and selection for this account, then start a new deployment."
		case "cannot name deployment release without creation time":
			return "The deployment is missing the information needed to prepare its release. Start a new deployment."
		}
	}
	return "Deployment preparation failed. Check the site's repository and deployment settings, then start a new deployment."
}

func (j *DeployJob) buildDeployConfig(ctx context.Context, site *models.Site, deployment *models.Deployment, teamID string) (tasks.DeployOptions, error) {
	repositoryURL := ""
	var hasAppAuth bool
	var tempToken, authURL, appName string

	grant, err := siteaccess.ForDeployment(ctx, j.Deps.DB, j.Deps.ProviderFactory, site, deployment, gitaccess.Clone)
	if err != nil {
		return tasks.DeployOptions{}, err
	}
	var credentials []string
	var grants []*gitaccess.RepositoryAccess
	if grant != nil {
		grants = append(grants, grant)
		repo, sc := grant.Repository, grant.SourceControl
		tempToken, authURL, appName = j.getAppAuthToken(grant.Context(ctx), sc, repo)
		if tempToken == "" {
			return tasks.DeployOptions{}, fmt.Errorf("cannot obtain selected repository credentials")
		}
		hasAppAuth = true
		repositoryURL = grant.CloneURL
		credentials = append(credentials, authURL, strings.TrimSuffix(authURL, ".git"))
		// Dependencies are the exact set captured when this deployment was requested.
		raw, _ := deployment.GitAccess["dependencies"].(map[string]any)
		for id, value := range raw {
			data, err := json.Marshal(value)
			if err != nil {
				return tasks.DeployOptions{}, err
			}
			var expected gitaccess.Snapshot
			if err := json.Unmarshal(data, &expected); err != nil {
				return tasks.DeployOptions{}, err
			}
			dep, err := siteaccess.ResolveDependency(ctx, j.Deps.DB, j.Deps.ProviderFactory, site.TeamID, id, &expected)
			if err != nil {
				return tasks.DeployOptions{}, err
			}
			grants = append(grants, dep)
			token, url, _ := j.getAppAuthToken(dep.Context(ctx), dep.SourceControl, dep.Repository)
			if token == "" {
				return tasks.DeployOptions{}, fmt.Errorf("cannot obtain selected dependency credentials")
			}
			credentials = append(credentials, url, strings.TrimSuffix(url, ".git"))
		}
		if err := grant.Validate(ctx); err != nil {
			return tasks.DeployOptions{}, err
		}
	} else if site.SourceControlRepositoriesID != nil {
		repo, sc := j.getRepositoryAndSourceControl(ctx, site)
		if repo == nil || sc == nil {
			return tasks.DeployOptions{}, fmt.Errorf("repository connection is unavailable")
		}
		repositoryURL = repo.SSHURL
	}

	// Use the persisted release name for zero-downtime deployment.
	var releaseTimestamp string
	if j.Payload.ZeroDowntime && deployment.ReleaseName != nil {
		releaseTimestamp = *deployment.ReleaseName
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
		BeforeExecution: func(ctx context.Context) error {
			for _, grant := range grants {
				if err := grant.Validate(ctx); err != nil {
					return err
				}
			}
			return nil
		},
		Site:             site,
		Deployment:       deployment,
		TeamID:           teamID,
		ServerName:       j.server.Name,
		DeploymentURL:    deploymentLogURL(j.Deps.FrontendURL, site.ServerID, site.ID, deployment.ID),
		RepositoryURL:    repositoryURL,
		HasAppAuth:       hasAppAuth,
		TempToken:        tempToken,
		AuthURL:          authURL,
		GitCredentials:   strings.Join(credentials, "\n") + "\n",
		AppName:          appName,
		ReleaseTimestamp: releaseTimestamp,
		EnvVariables:     envVars,
	}, nil
}

func (j *DeployJob) getRepositoryAndSourceControl(ctx context.Context, site *models.Site) (*gitmodels.SourceControlRepository, *gitmodels.SourceControl) {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return nil, nil
	}

	// Query repository from source_control_repositories table
	var repo gitmodels.SourceControlRepository
	if err := j.Deps.DB.WithContext(ctx).First(&repo, "id = ? AND source_control_id = ?", *site.SourceControlRepositoriesID, *site.SourceControlID).Error; err != nil {
		return nil, nil
	}

	// Get source control
	var sourceControl *gitmodels.SourceControl
	if j.Deps.SourceControlRepo != nil {
		sc, err := j.Deps.SourceControlRepo.FindByID(ctx, *site.SourceControlID)
		if err == nil && sc.TeamID == site.TeamID {
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

func (j *DeployJob) handleDeploymentFailure(ctx context.Context, message string) {
	var site models.Site
	var deployment models.Deployment
	changed := false
	err := j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Read the scope first, then lock server -> site -> deployment so ownership
		// cannot change between validation and attaching the diagnostic log.
		var scope models.Site
		if err := tx.Select("id", "server_id", "team_id").First(&scope, "id = ?", j.Payload.SiteID).Error; err != nil {
			return err
		}
		var server servermodels.Server
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "team_id").
			Where("id = ? AND team_id = ?", scope.ServerID, scope.TeamID).First(&server).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND server_id = ? AND team_id = ?", scope.ID, server.ID, server.TeamID).First(&site).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND site_id = ? AND team_id = ?", j.Payload.DeploymentID, site.ID, site.TeamID).First(&deployment).Error; err != nil {
			return err
		}
		if (j.site != nil && (j.site.ID != site.ID || j.site.ServerID != site.ServerID || j.site.TeamID != site.TeamID)) ||
			(j.server != nil && (j.server.ID != server.ID || j.server.TeamID != server.TeamID)) ||
			(j.deployment != nil && (j.deployment.ID != deployment.ID || j.deployment.SiteID != site.ID || j.deployment.TeamID != site.TeamID)) {
			return errors.New("deployment ownership changed before recording failure")
		}
		// A task callback may already have handled this attempt. Never backfill
		// terminal history or advance the deployment queue a second time.
		if deployment.Status != sitetypes.DeploymentStatusPending && deployment.Status != sitetypes.DeploymentStatusInstalling {
			return nil
		}
		fields := map[string]any{"status": sitetypes.DeploymentStatusFailed}
		if deployment.TaskID == nil || *deployment.TaskID == "" {
			task := servermodels.Task{
				Name: "Prepare site deployment", Type: "site:deployment_preparation",
				User: site.User, Status: "failed",
				SiteID: &site.ID, Output: dbtype.EncryptedString(message),
			}
			task.ServerID = server.ID
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			fields["task_id"] = task.ID
			deployment.TaskID = &task.ID
		} else {
			var task servermodels.Task
			if err := tx.Select("id").Where("id = ? AND server_id = ? AND (site_id IS NULL OR site_id = ?)", *deployment.TaskID, server.ID, site.ID).
				First(&task).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&models.Deployment{}).
			Where("id = ? AND site_id = ? AND team_id = ?", deployment.ID, site.ID, site.TeamID).Updates(fields).Error; err != nil {
			return err
		}
		if site.InstalledAt == nil {
			if err := tx.Model(&models.Site{}).Where("id = ? AND server_id = ? AND team_id = ?", site.ID, server.ID, site.TeamID).
				Update("installation_failed_at", time.Now().UTC()).Error; err != nil {
				return err
			}
		}
		deployment.Status = sitetypes.DeploymentStatusFailed
		changed = true
		return nil
	})
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to record deployment preparation failure")
		return
	}
	if !changed {
		return
	}

	// Update deployment status on git provider
	j.updateProviderDeploymentStatus(ctx, &site, &deployment, gitproviders.DeploymentStatusFailure)

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
	if _, err := j.Deps.DeploymentDispatcher().AdvanceNext(ctx, siteID); err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", siteID).Msg("Failed to enqueue next deployment")
	}
}

// createProviderDeployment creates a deployment on the git provider
func (j *DeployJob) createProviderDeployment(ctx context.Context, site *models.Site, serverID string, deployment *models.Deployment) {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return
	}

	repo, sourceControl := j.getRepositoryAndSourceControl(ctx, site)
	if repo == nil || sourceControl == nil {
		return
	}

	if j.Deps.ProviderFactory == nil {
		return
	}

	grant, accessErr := siteaccess.ForDeployment(ctx, j.Deps.DB, j.Deps.ProviderFactory, site, deployment, gitaccess.Deployment)
	if accessErr != nil {
		return
	}
	if grant != nil {
		ctx = grant.Context(ctx)
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
		LogURL:       deploymentLogURL(j.Deps.FrontendURL, serverID, site.ID, deployment.ID),
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

	repo, sourceControl := j.getRepositoryAndSourceControl(ctx, site)
	if repo == nil || sourceControl == nil {
		return
	}

	if j.Deps.ProviderFactory == nil {
		return
	}

	grant, accessErr := siteaccess.ForDeployment(ctx, j.Deps.DB, j.Deps.ProviderFactory, site, deployment, gitaccess.Deployment)
	if accessErr != nil {
		return
	}
	if grant != nil {
		ctx = grant.Context(ctx)
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
		LogURL:       deploymentLogURL(j.Deps.FrontendURL, site.ServerID, site.ID, deployment.ID),
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
	return deployments.NewTask(siteID, deploymentID, userID, envVars, false)
}

// NewDeployZeroDowntimeTask creates a zero-downtime deploy job
// Uses TaskID for deduplication to prevent the same deployment from running multiple times
func NewDeployZeroDowntimeTask(siteID, deploymentID string, userID string, envVars map[string]string) (*asynq.Task, error) {
	return deployments.NewTask(siteID, deploymentID, userID, envVars, true)
}
