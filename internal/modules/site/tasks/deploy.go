package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/notification/notifications"
	siteevents "github.com/kkz6/launch-go/internal/modules/site/events"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// Task type constants for deployment operations
const (
	// DeploySiteTaskType is the registered type name for callback reconstruction
	DeploySiteTaskType            = "site:deploy"
	RollbackDeploymentTaskType    = "site:rollback_deployment"
	postDeploymentEventMaxRetries = 3
)

// DeployOptions holds options for deploying a site
type DeployOptions struct {
	Site       *models.Site
	Deployment *models.Deployment

	// TeamID for broadcasting events (from server.TeamID)
	TeamID string

	// Server name for notifications
	ServerName string

	// Deployment URL for the notification action
	DeploymentURL string

	// Git authentication (set by job after checking source control)
	RepositoryURL string
	HasAppAuth    bool
	TempToken     string
	AuthURL       string
	AppName       string

	// Zero-downtime specific
	ReleaseTimestamp string

	// Environment variables (for first deployment)
	EnvVariables map[string]string
}

// callbackData holds data needed for callback handling (serialized to instance field)
type callbackData struct {
	SiteID                      string `json:"site_id"`
	ServerID                    string `json:"server_id"`
	TeamID                      string `json:"team_id"`
	DeploymentID                string `json:"deployment_id"`
	SiteType                    string `json:"site_type"`
	IsFirstDeploy               bool   `json:"is_first_deploy"`
	QueueDeployments            bool   `json:"queue_deployments"`
	ZeroDowntimeDeployment      bool   `json:"zero_downtime_deployment"`
	DeploymentReleasesRetention int    `json:"deployment_releases_retention"`

	// For notifications
	SiteAddress   string `json:"site_address"`
	ServerName    string `json:"server_name"`
	DeploymentURL string `json:"deployment_url"`
}

// deploySiteTask implements Task and CallbackPayload interfaces.
type deploySiteTask struct {
	*taskrunner.BaseTask
	opts     DeployOptions
	callback callbackData
}

// DeploySiteTask creates a new deployment task with callback support.
// Returns taskrunner.Task interface to satisfy the linter while preserving
// callback functionality through type assertions internally.
func DeploySiteTask(opts DeployOptions) taskrunner.Task {
	var name, script string

	if opts.Site.ZeroDowntimeDeployment {
		name = "Deploy Site (Zero Downtime)"
		script = buildZeroDowntimeScript(opts)
	} else {
		name = "Deploy Site"
		script = buildStandardScript(opts)
	}

	return &deploySiteTask{
		BaseTask: taskrunner.NewBaseTask(
			taskrunner.WithName(name),
			taskrunner.WithScript(script),
			taskrunner.WithTimeoutSeconds(600),
		),
		opts: opts,
		callback: callbackData{
			SiteID:                      opts.Site.ID,
			ServerID:                    opts.Site.ServerID,
			TeamID:                      opts.TeamID,
			DeploymentID:                opts.Deployment.ID,
			SiteType:                    string(opts.Site.Type),
			IsFirstDeploy:               opts.Site.InstalledAt == nil,
			QueueDeployments:            opts.Site.QueueDeployments,
			ZeroDowntimeDeployment:      opts.Site.ZeroDowntimeDeployment,
			DeploymentReleasesRetention: opts.Site.DeploymentReleasesRetention,
			SiteAddress:                 opts.Site.Address,
			ServerName:                  opts.ServerName,
			DeploymentURL:               opts.DeploymentURL,
		},
	}
}

// TypeName returns the registered type name for reconstruction
func (t *deploySiteTask) TypeName() string {
	return DeploySiteTaskType
}

// MarshalPayload returns JSON representation of the task state needed for callbacks
func (t *deploySiteTask) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.callback)
}

// OnSuccess is called when the task completes successfully
func (t *deploySiteTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Info().
			Str("task_id", taskID).
			Str("site_id", t.callback.SiteID).
			Str("deployment_id", t.callback.DeploymentID).
			Msg("DeploySite: onFinished callback triggered")
	}

	// Update deployment status to finished
	if err := cbCtx.DB.Model(&models.Deployment{}).
		Where("id = ?", t.callback.DeploymentID).
		Update("status", sitetypes.DeploymentStatusFinished).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	t.broadcastDeploymentEvent(cbCtx, "deployment.finished", "finished", nil)
	t.broadcastDeploymentEvent(cbCtx, "deployment.progress", "finished", map[string]interface{}{"message": "Deployment finished"})

	// Update deployment status on git provider (GitHub/GitLab deployment status)
	t.dispatchUpdateProviderDeploymentStatus(cbCtx, "success")

	// Publish one durable, deployment-scoped domain event. Independently
	// registered listeners fan out post-deployment actions (queue restarts
	// today; more actions can be added without coupling them to this callback).
	if err := t.dispatchDeploymentSucceeded(cbCtx, taskID); err != nil && cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Err(err).
			Str("deployment_id", t.callback.DeploymentID).
			Msg("failed to publish deployment succeeded event")
	}

	// If first deployment, dispatch InstallCaddyfile job and install pending queues
	if t.callback.IsFirstDeploy {
		t.dispatchInstallCaddyfile(cbCtx)
		t.installPendingQueues(ctx, cbCtx)
	}

	// Analyze Laravel features for Laravel sites
	if t.callback.SiteType == string(sitetypes.SiteTypeLaravel) {
		t.dispatchAnalyzeLaravelFeatures(cbCtx)
	}

	// Process next queued deployment if enabled
	if t.callback.QueueDeployments {
		t.processNextQueuedDeployment(ctx, cbCtx)
	}

	// Cleanup old deployment records
	t.cleanupOldDeployments(ctx, cbCtx)

	return nil
}

// OnFailure is called when the task fails with an exit code
func (t *deploySiteTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("site_id", t.callback.SiteID).
			Str("deployment_id", t.callback.DeploymentID).
			Int("exit_code", exitCode).
			Msg("DeploySite: onFailed callback triggered")
	}

	// Update deployment status to failed
	if err := cbCtx.DB.Model(&models.Deployment{}).
		Where("id = ?", t.callback.DeploymentID).
		Update("status", sitetypes.DeploymentStatusFailed).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	t.broadcastDeploymentEvent(cbCtx, "deployment.failed", "failed", map[string]interface{}{"exit_code": exitCode})
	t.broadcastDeploymentEvent(cbCtx, "deployment.progress", "failed", map[string]interface{}{"message": "Deployment failed"})

	// Update deployment status on git provider
	t.dispatchUpdateProviderDeploymentStatus(cbCtx, "failure")

	// If first deployment, mark site installation as failed
	if t.callback.IsFirstDeploy {
		cbCtx.DB.Model(&models.Site{}).
			Where("id = ?", t.callback.SiteID).
			Update("installation_failed_at", time.Now())
	}

	// Send notification based on whether this is first deployment or not
	t.sendDeploymentNotification(ctx, cbCtx, taskID, notifications.DeploymentStatusFailed)

	// Process next queued deployment if enabled
	if t.callback.QueueDeployments {
		t.processNextQueuedDeployment(ctx, cbCtx)
	}

	return nil
}

// OnExpired is called when the task times out
func (t *deploySiteTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("site_id", t.callback.SiteID).
			Str("deployment_id", t.callback.DeploymentID).
			Msg("DeploySite: onTimeout callback triggered")
	}

	// Update deployment status to timeout
	if err := cbCtx.DB.Model(&models.Deployment{}).
		Where("id = ?", t.callback.DeploymentID).
		Update("status", sitetypes.DeploymentStatusTimeout).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	t.broadcastDeploymentEvent(cbCtx, "deployment.timeout", "timeout", nil)
	t.broadcastDeploymentEvent(cbCtx, "deployment.progress", "timeout", map[string]interface{}{"message": "Deployment timed out"})

	// Update deployment status on git provider
	t.dispatchUpdateProviderDeploymentStatus(cbCtx, "failure")

	// Send notification
	t.sendDeploymentNotification(ctx, cbCtx, taskID, notifications.DeploymentStatusTimeout)

	// Process next queued deployment if enabled
	if t.callback.QueueDeployments {
		t.processNextQueuedDeployment(ctx, cbCtx)
	}

	return nil
}

// broadcastDeploymentEvent broadcasts a deployment event with the standard fields
func (t *deploySiteTask) broadcastDeploymentEvent(cbCtx *taskrunner.CallbackContext, event, status string, extra map[string]interface{}) {
	data := map[string]interface{}{
		"team_id":       t.callback.TeamID,
		"site_id":       t.callback.SiteID,
		"server_id":     t.callback.ServerID,
		"deployment_id": t.callback.DeploymentID,
		"status":        status,
	}

	for k, v := range extra {
		data[k] = v
	}

	cbCtx.BroadcastToTeam(t.callback.TeamID, event, data)
}

// sendDeploymentNotification sends the appropriate notification for deployment failure
func (t *deploySiteTask) sendDeploymentNotification(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, status notifications.DeploymentStatus) {
	// Get task output
	output := cbCtx.GetTaskOutputTail(taskID, 30)

	// Get deployment for git info
	var deployment models.Deployment
	cbCtx.DB.First(&deployment, "id = ?", t.callback.DeploymentID)

	if t.callback.IsFirstDeploy {
		// Site installation failed
		notif := notifications.NewSiteInstallationFailedNotification(t.callback.SiteAddress, t.callback.ServerName)
		if deployment.GitHash != nil && *deployment.GitHash != "" {
			notif.WithGitInfo(*deployment.GitHash, deployment.CommitMessage())
		}
		notif.WithOutput(output)
		notif.WithSiteURL(t.callback.DeploymentURL)

		if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
			if cbCtx.Logger != nil {
				cbCtx.Logger.Warn().Err(err).Msg("Failed to send site installation failed notification")
			}
		}
	} else {
		// Deployment failed
		notif := notifications.NewDeploymentFailedNotification(t.callback.SiteAddress, t.callback.ServerName, status)
		if deployment.GitHash != nil && *deployment.GitHash != "" {
			notif.WithGitInfo(*deployment.GitHash, deployment.CommitMessage(), deployment.CommitAuthor())
		}
		if deployment.UserID != nil && *deployment.UserID != "" {
			var triggeredBy struct {
				Name string
			}
			if err := cbCtx.DB.Table("users").Select("name").Where("id = ?", *deployment.UserID).Take(&triggeredBy).Error; err == nil {
				notif.WithTriggeredBy(triggeredBy.Name)
			}
		}
		notif.WithOutput(output)
		notif.WithSiteURL(t.callback.DeploymentURL)
		if deployment.CreatedAt != nil {
			notif.WithDeploymentTime(*deployment.CreatedAt)
		}

		if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
			if cbCtx.Logger != nil {
				cbCtx.Logger.Warn().Err(err).Msg("Failed to send deployment failed notification")
			}
		}
	}
}

// dispatchJob is a helper to dispatch an asynq job with MaxRetry(0) to prevent retries
func (t *deploySiteTask) dispatchJob(cbCtx *taskrunner.CallbackContext, jobType string, payload any, taskID string) {
	if cbCtx.Queue == nil {
		return
	}
	data, _ := json.Marshal(payload)
	opts := []asynq.Option{asynq.MaxRetry(0)}
	if taskID != "" {
		opts = append(opts, asynq.TaskID(taskID))
	}
	task := asynq.NewTask(jobType, data, opts...)
	if _, err := cbCtx.Queue.Enqueue(task); err != nil && cbCtx.Logger != nil {
		cbCtx.Logger.Error().Err(err).Str("job_type", jobType).Msg("Failed to dispatch job")
	}
}

// Typed job payload structs - mirror the job payload types for compile-time safety.
// These match the payload structs in internal/modules/site/jobs/*.go

type updateProviderDeploymentStatusPayload struct {
	SiteID       string `json:"site_id"`
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"`
}

// dispatchUpdateProviderDeploymentStatus dispatches a job to update deployment status on the git provider.
func (t *deploySiteTask) dispatchUpdateProviderDeploymentStatus(cbCtx *taskrunner.CallbackContext, status string) {
	t.dispatchJob(cbCtx, "site:update_provider_deployment_status", updateProviderDeploymentStatusPayload{
		SiteID:       t.callback.SiteID,
		DeploymentID: t.callback.DeploymentID,
		Status:       status,
	}, "")
}

func (t *deploySiteTask) dispatchDeploymentSucceeded(
	cbCtx *taskrunner.CallbackContext,
	taskID string,
) error {
	event, err := siteevents.NewDeploymentSucceeded(siteevents.DeploymentSucceeded{
		DeploymentID: t.callback.DeploymentID,
		SiteID:       t.callback.SiteID,
		ServerID:     t.callback.ServerID,
		TeamID:       t.callback.TeamID,
		TaskID:       taskID,
	})
	if err != nil {
		return err
	}

	eventTaskID := deploymentEventTaskID(event.Name, event.ID)
	err = cbCtx.DispatchJobWithOptions(
		siteevents.TypeProcessEvent,
		event,
		asynq.TaskID(eventTaskID),
		asynq.MaxRetry(postDeploymentEventMaxRetries),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
		return nil
	}
	return err
}

func deploymentEventTaskID(eventName, eventID string) string {
	return pkgjobs.Dedup("site-event", eventName, eventID)
}

type caddyfilePayload struct {
	SiteID string  `json:"site_id"`
	UserID *string `json:"user_id,omitempty"`
}

type analyzeLaravelFeaturesPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// dispatchInstallCaddyfile dispatches the install caddyfile job with type-safe payload.
func (t *deploySiteTask) dispatchInstallCaddyfile(cbCtx *taskrunner.CallbackContext) {
	t.dispatchJob(cbCtx, "site:install_caddyfile", caddyfilePayload{
		SiteID: t.callback.SiteID,
	}, fmt.Sprintf("install_caddyfile:%s", t.callback.SiteID))
}

// dispatchAnalyzeLaravelFeatures dispatches the analyze Laravel features job with type-safe payload.
func (t *deploySiteTask) dispatchAnalyzeLaravelFeatures(cbCtx *taskrunner.CallbackContext) {
	t.dispatchJob(cbCtx, "site:analyze_laravel_features", analyzeLaravelFeaturesPayload{
		SiteID:   t.callback.SiteID,
		ServerID: t.callback.ServerID,
	}, fmt.Sprintf("analyze_features:%s", t.callback.SiteID))
}

// processNextQueuedDeployment finds and dispatches the next queued deployment
func (t *deploySiteTask) processNextQueuedDeployment(ctx context.Context, cbCtx *taskrunner.CallbackContext) {
	var nextDeployment models.Deployment
	err := cbCtx.DB.Where("site_id = ? AND status = ?", t.callback.SiteID, sitetypes.DeploymentStatusQueued).
		Order("created_at ASC").
		First(&nextDeployment).Error

	if err != nil {
		return
	}

	// Update status to pending
	cbCtx.DB.Model(&nextDeployment).Update("status", sitetypes.DeploymentStatusPending)

	// Get site to check zero downtime setting
	var site models.Site
	if err := cbCtx.DB.First(&site, "id = ?", t.callback.SiteID).Error; err != nil {
		return
	}

	jobType := "site:deploy"
	taskID := fmt.Sprintf("deploy:%s", nextDeployment.ID)
	if site.ZeroDowntimeDeployment {
		jobType = "site:deploy_zero_downtime"
		taskID = fmt.Sprintf("deploy_zd:%s", nextDeployment.ID)
	}

	t.dispatchJob(cbCtx, jobType, map[string]string{
		"site_id":       t.callback.SiteID,
		"deployment_id": nextDeployment.ID,
	}, taskID)
}

// installPendingQueues installs any queues that were created before the first deployment
func (t *deploySiteTask) installPendingQueues(ctx context.Context, cbCtx *taskrunner.CallbackContext) {
	var queues []models.Queue
	cbCtx.DB.Where("site_id = ? AND installed_at IS NULL AND installation_failed_at IS NULL", t.callback.SiteID).Find(&queues)

	for _, q := range queues {
		t.dispatchJob(cbCtx, "site:install_queue", map[string]string{
			"site_id":  t.callback.SiteID,
			"queue_id": q.ID,
		}, fmt.Sprintf("install_queue:%s:%s", t.callback.SiteID, q.ID))
	}
}

// cleanupOldDeployments removes old deployment records beyond the retention limit.
// For zero-downtime deployments, it uses the site's DeploymentReleasesRetention setting.
// For normal deployments, it keeps only the most recent 5 deployments.
func (t *deploySiteTask) cleanupOldDeployments(ctx context.Context, cbCtx *taskrunner.CallbackContext) {
	// Determine retention count based on deployment type
	retentionCount := 5 // default for normal deployments
	if t.callback.ZeroDowntimeDeployment {
		retentionCount = t.callback.DeploymentReleasesRetention
		if retentionCount <= 0 {
			retentionCount = 10 // default if not set
		}
	}

	// Get IDs of deployments to keep (most recent N)
	var deploymentsToKeep []string
	err := cbCtx.DB.Model(&models.Deployment{}).
		Select("id").
		Where("site_id = ?", t.callback.SiteID).
		Order("created_at DESC").
		Limit(retentionCount).
		Pluck("id", &deploymentsToKeep).Error
	if err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Error().Err(err).
				Str("site_id", t.callback.SiteID).
				Msg("Failed to get deployments to keep for cleanup")
		}
		return
	}

	// If we have fewer deployments than retention, nothing to delete
	if len(deploymentsToKeep) < retentionCount {
		return
	}

	// Delete deployments not in the keep list
	result := cbCtx.DB.Where("site_id = ? AND id NOT IN ?", t.callback.SiteID, deploymentsToKeep).
		Delete(&models.Deployment{})
	if result.Error != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Error().Err(result.Error).
				Str("site_id", t.callback.SiteID).
				Msg("Failed to cleanup old deployments")
		}
		return
	}

	if result.RowsAffected > 0 && cbCtx.Logger != nil {
		cbCtx.Logger.Info().
			Int64("deleted_count", result.RowsAffected).
			Int("retention_count", retentionCount).
			Bool("zero_downtime", t.callback.ZeroDowntimeDeployment).
			Str("site_id", t.callback.SiteID).
			Msg("Cleaned up old deployment records")
	}
}

// NewTask implements taskrunner.CallbackStateFactory.
// Creates a deploySiteTask from the serialized state for callback handling.
func (s callbackData) NewTask() taskrunner.CallbackHandler {
	return &deploySiteTask{
		BaseTask: taskrunner.NewBaseTask(),
		callback: s,
	}
}

// buildStandardScript generates the deployment script for standard deployment
func buildStandardScript(opts DeployOptions) string {
	site := opts.Site
	repoDir := fmt.Sprintf("%s/repository", site.Path)
	logsDir := site.GetLogsDirectory()
	phpBinary := site.GetPhpBinary()

	var scriptBuilder strings.Builder

	scriptBuilder.WriteString("#!/bin/bash\n")
	scriptBuilder.WriteString(templates.ShellDefaultsLenient())
	scriptBuilder.WriteString("\n\n")

	scriptBuilder.WriteString(templates.MustRender("site", "deployment/shell_variables.sh", struct {
		PHPBinary string
	}{phpBinary}))
	scriptBuilder.WriteString("\n")

	scriptBuilder.WriteString("# Create the necessary directories\n")
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", repoDir))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", logsDir))
	scriptBuilder.WriteString("\n")

	if site.InstalledAt != nil && site.HookBeforeUpdatingRepository != nil && *site.HookBeforeUpdatingRepository != "" {
		scriptBuilder.WriteString("echo \"Running hook before updating repository\"\n")
		scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", repoDir))
		scriptBuilder.WriteString(*site.HookBeforeUpdatingRepository + "\n\n")
	}

	if opts.RepositoryURL != "" {
		scriptBuilder.WriteString(renderUpdateRepository(opts, repoDir, ""))
		scriptBuilder.WriteString("\n")

		if site.HookAfterUpdatingRepository != nil && *site.HookAfterUpdatingRepository != "" {
			scriptBuilder.WriteString("echo \"Running hook after updating repository\"\n")
			scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", repoDir))
			scriptBuilder.WriteString(*site.HookAfterUpdatingRepository + "\n\n")
		}
	}

	if site.InstalledAt == nil {
		scriptBuilder.WriteString(renderPrepareFreshInstallation(opts, repoDir, "", ""))
		scriptBuilder.WriteString("\n")
	}

	if site.InstalledAt != nil && (site.Type == sitetypes.SiteTypeWordpress || site.Type == sitetypes.SiteTypePhpMyAdmin) {
		scriptBuilder.WriteString("echo \"Application already installed!\"\n\n")
	}

	scriptBuilder.WriteString("echo \"Done!\"\n")

	return scriptBuilder.String()
}

// buildZeroDowntimeScript generates the deployment script for zero-downtime deployment
func buildZeroDowntimeScript(opts DeployOptions) string {
	site := opts.Site
	phpBinary := site.GetPhpBinary()

	// Derive directories from site
	repoDir := fmt.Sprintf("%s/repository", site.Path)
	sharedDir := fmt.Sprintf("%s/shared", site.Path)
	releasesDir := fmt.Sprintf("%s/releases", site.Path)
	releaseDir := fmt.Sprintf("%s/%s", releasesDir, opts.ReleaseTimestamp)
	currentDir := fmt.Sprintf("%s/current", site.Path)
	logsDir := site.GetLogsDirectory()

	var scriptBuilder strings.Builder

	scriptBuilder.WriteString("#!/bin/bash\n")
	scriptBuilder.WriteString(templates.ShellDefaultsLenient())
	scriptBuilder.WriteString("\n\n")

	scriptBuilder.WriteString(templates.MustRender("site", "deployment/shell_variables.sh", struct {
		PHPBinary string
	}{phpBinary}))
	scriptBuilder.WriteString("\n")

	scriptBuilder.WriteString("# Create the necessary directories\n")
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", repoDir))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", sharedDir))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", releaseDir))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", logsDir))
	scriptBuilder.WriteString("\n")

	scriptBuilder.WriteString("# Cleanup old releases\n")
	scriptBuilder.WriteString(templates.MustRender("site", "deployment/cleanup_old_releases.sh", struct {
		LatestDeploymentTimestamp string
		ReleasesDirectory         string
		RetentionCount            int
	}{
		LatestDeploymentTimestamp: opts.ReleaseTimestamp,
		ReleasesDirectory:         releasesDir,
		RetentionCount:            site.DeploymentReleasesRetention,
	}))
	scriptBuilder.WriteString("\n")

	if site.HookBeforeUpdatingRepository != nil && *site.HookBeforeUpdatingRepository != "" {
		scriptBuilder.WriteString("echo \"Running hook before updating repository\"\n")
		scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", releaseDir))
		scriptBuilder.WriteString(*site.HookBeforeUpdatingRepository + "\n\n")
	}

	if opts.RepositoryURL != "" {
		scriptBuilder.WriteString(renderUpdateRepository(opts, repoDir, releaseDir))
		scriptBuilder.WriteString("\n")

		if site.HookAfterUpdatingRepository != nil && *site.HookAfterUpdatingRepository != "" {
			scriptBuilder.WriteString("echo \"Running hook after updating repository\"\n")
			scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", releaseDir))
			scriptBuilder.WriteString(*site.HookAfterUpdatingRepository + "\n\n")
		}
	}

	if site.InstalledAt == nil {
		scriptBuilder.WriteString(renderPrepareFreshInstallation(opts, repoDir, releaseDir, sharedDir))
		scriptBuilder.WriteString("\n")
	}

	scriptBuilder.WriteString("# Link shared directories\n")
	scriptBuilder.WriteString(templates.MustRender("site", "deployment/link_shared_directories.sh", struct {
		SharedDirectories []string
		SharedDirectory   string
		ReleaseDirectory  string
	}{
		SharedDirectories: site.SharedDirectories,
		SharedDirectory:   sharedDir,
		ReleaseDirectory:  releaseDir,
	}))
	scriptBuilder.WriteString("\n")

	scriptBuilder.WriteString("# Link shared files\n")
	scriptBuilder.WriteString(templates.MustRender("site", "deployment/link_shared_files.sh", struct {
		SharedFiles      []string
		SharedDirectory  string
		ReleaseDirectory string
	}{
		SharedFiles:      site.SharedFiles,
		SharedDirectory:  sharedDir,
		ReleaseDirectory: releaseDir,
	}))
	scriptBuilder.WriteString("\n")

	scriptBuilder.WriteString("# Make directories writable\n")
	scriptBuilder.WriteString(templates.MustRender("site", "deployment/make_directories_writable.sh", struct {
		WritableDirectories []string
		ReleaseDirectory    string
		Username            string
	}{
		WritableDirectories: site.WriteableDirectories,
		ReleaseDirectory:    releaseDir,
		Username:            site.User,
	}))
	scriptBuilder.WriteString("\n")

	if site.HookBeforeMakingCurrent != nil && *site.HookBeforeMakingCurrent != "" {
		scriptBuilder.WriteString("echo \"Running hook before putting the site live\"\n")
		scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", releaseDir))
		scriptBuilder.WriteString(*site.HookBeforeMakingCurrent + "\n\n")
	}

	scriptBuilder.WriteString("# Make deployment current\n")
	scriptBuilder.WriteString(templates.MustRender("site", "deployment/make_deployment_current.sh", struct {
		SitePath         string
		ReleaseDirectory string
		CurrentDirectory string
	}{
		SitePath:         site.Path,
		ReleaseDirectory: releaseDir,
		CurrentDirectory: currentDir,
	}))
	scriptBuilder.WriteString("\n")

	if site.HookAfterMakingCurrent != nil && *site.HookAfterMakingCurrent != "" {
		scriptBuilder.WriteString("echo \"Running hook after putting the site live\"\n")
		scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", releaseDir))
		scriptBuilder.WriteString(*site.HookAfterMakingCurrent + "\n\n")
	}

	scriptBuilder.WriteString("echo \"Done!\"\n")

	return scriptBuilder.String()
}

// renderUpdateRepository renders the update repository template
func renderUpdateRepository(opts DeployOptions, repoDir, releaseDir string) string {
	site := opts.Site
	return templates.MustRender("site", "deployment/update_repository.sh", struct {
		RepositoryDirectory    string
		RepositoryURL          string
		RepositoryBranch       string
		SitePath               string
		ZeroDowntimeDeployment bool
		DeploymentID           string
		HasAppAuth             bool
		TempToken              string
		AuthURL                string
		DeployKeyPrivate       string
		AppName                string
		ReleaseDirectory       string
	}{
		RepositoryDirectory:    repoDir,
		RepositoryURL:          opts.RepositoryURL,
		RepositoryBranch:       site.GetRepositoryBranch(),
		SitePath:               site.Path,
		ZeroDowntimeDeployment: site.ZeroDowntimeDeployment,
		DeploymentID:           opts.Deployment.ID,
		HasAppAuth:             opts.HasAppAuth,
		TempToken:              opts.TempToken,
		AuthURL:                opts.AuthURL,
		DeployKeyPrivate:       string(site.DeployKeyPrivate),
		AppName:                opts.AppName,
		ReleaseDirectory:       releaseDir,
	})
}

// renderPrepareFreshInstallation renders the prepare fresh installation template
func renderPrepareFreshInstallation(opts DeployOptions, repoDir, releaseDir, sharedDir string) string {
	site := opts.Site
	var scriptBuilder strings.Builder
	scriptBuilder.WriteString(fmt.Sprintf("cd %s\n\n", site.Path))

	switch site.Type {
	case sitetypes.SiteTypeLaravel:
		scriptBuilder.WriteString(templates.MustRender("site", "deployment/prepare_fresh_installation/laravel.sh", struct {
			SitePath               string
			ZeroDowntimeDeployment bool
			SharedDirectory        string
			ReleaseDirectory       string
			RepositoryDirectory    string
			EnvVariables           map[string]string
		}{
			SitePath:               site.Path,
			ZeroDowntimeDeployment: site.ZeroDowntimeDeployment,
			SharedDirectory:        sharedDir,
			ReleaseDirectory:       releaseDir,
			RepositoryDirectory:    repoDir,
			EnvVariables:           opts.EnvVariables,
		}))

	case sitetypes.SiteTypeWordpress:
		scriptBuilder.WriteString(templates.MustRender("site", "deployment/prepare_fresh_installation/wordpress.sh", struct {
			SitePath            string
			RepositoryDirectory string
			EnvVariables        map[string]string
		}{
			SitePath:            site.Path,
			RepositoryDirectory: repoDir,
			EnvVariables:        opts.EnvVariables,
		}))

	case sitetypes.SiteTypePhpMyAdmin:
		phpVersion := sitetypes.PhpVersion84
		if site.PhpVersion != nil {
			phpVersion = *site.PhpVersion
		}

		scriptBuilder.WriteString(templates.MustRender("site", "deployment/prepare_fresh_installation/phpmyadmin.sh", struct {
			SitePath            string
			RepositoryDirectory string
			DownloadURL         string
			Version             string
		}{
			SitePath:            site.Path,
			RepositoryDirectory: repoDir,
			DownloadURL:         phpVersion.GetPhpMyAdminDownloadURL(),
			Version:             phpVersion.GetPhpMyAdminVersion(),
		}))
	}

	scriptBuilder.WriteString(fmt.Sprintf("\ncd %s\n", site.Path))
	return scriptBuilder.String()
}

// RollbackDeploymentConfig holds configuration for rolling back a deployment
type RollbackDeploymentConfig struct {
	SitePath         string
	ReleaseDirectory string
	CurrentDirectory string
}

// RollbackDeployment creates a task to rollback to a previous deployment
func RollbackDeployment(config RollbackDeploymentConfig) *taskrunner.BaseTask {
	script := templates.MustRender("site", "rollback_deployment.sh", struct {
		SitePath         string
		ReleaseDirectory string
		CurrentDirectory string
	}{
		SitePath:         config.SitePath,
		ReleaseDirectory: config.ReleaseDirectory,
		CurrentDirectory: config.CurrentDirectory,
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Rollback Deployment"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}
