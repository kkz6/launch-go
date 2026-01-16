package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const (
	// DeploySiteTaskType is the registered type name for callback reconstruction
	DeploySiteTaskType = "site:deploy"
)

// DeployOptions holds options for deploying a site
type DeployOptions struct {
	Site       *models.Site
	Deployment *models.Deployment

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
	SiteID           string `json:"site_id"`
	ServerID         string `json:"server_id"`
	DeploymentID     string `json:"deployment_id"`
	SiteType         string `json:"site_type"`
	IsFirstDeploy    bool   `json:"is_first_deploy"`
	QueueDeployments bool   `json:"queue_deployments"`
	AutoRestartQueue bool   `json:"auto_restart_queue"`
}

// deploySiteTask implements Task and CallbackPayload interfaces.
type deploySiteTask struct {
	*taskrunner.BaseTask
	opts     DeployOptions
	callback callbackData
}

// DeploySiteTask creates a new deployment task with callback support.
func DeploySiteTask(opts DeployOptions) *deploySiteTask {
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
			SiteID:           opts.Site.ID,
			ServerID:         opts.Site.ServerID,
			DeploymentID:     opts.Deployment.ID,
			SiteType:         string(opts.Site.Type),
			IsFirstDeploy:    opts.Site.InstalledAt == nil,
			QueueDeployments: opts.Site.QueueDeployments,
			AutoRestartQueue: opts.Site.AutoRestartQueue,
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
		Update("status", enums.DeploymentStatusFinished).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// If first deployment, dispatch InstallCaddyfile job
	if t.callback.IsFirstDeploy {
		t.dispatchInstallCaddyfile(cbCtx)
	}

	// Analyze Laravel features for Laravel sites
	if t.callback.SiteType == string(enums.SiteTypeLaravel) {
		t.dispatchAnalyzeLaravelFeatures(cbCtx)
	}

	// Process next queued deployment if enabled
	if t.callback.QueueDeployments {
		t.processNextQueuedDeployment(ctx, cbCtx)
	}

	// Restart queue workers if auto-restart is enabled
	if t.callback.AutoRestartQueue {
		t.restartQueueWorkers(ctx, cbCtx)
	}

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
		Update("status", enums.DeploymentStatusFailed).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// If first deployment, mark site installation as failed
	if t.callback.IsFirstDeploy {
		cbCtx.DB.Model(&models.Site{}).
			Where("id = ?", t.callback.SiteID).
			Update("installation_failed_at", time.Now())
	}

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
		Update("status", enums.DeploymentStatusTimeout).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Process next queued deployment if enabled
	if t.callback.QueueDeployments {
		t.processNextQueuedDeployment(ctx, cbCtx)
	}

	return nil
}

// dispatchJob is a helper to dispatch an asynq job
func (t *deploySiteTask) dispatchJob(cbCtx *taskrunner.CallbackContext, jobType string, payload any) {
	if cbCtx.Queue == nil {
		return
	}
	data, _ := json.Marshal(payload)
	task := asynq.NewTask(jobType, data)
	if _, err := cbCtx.Queue.Enqueue(task); err != nil && cbCtx.Logger != nil {
		cbCtx.Logger.Error().Err(err).Str("job_type", jobType).Msg("Failed to dispatch job")
	}
}

// Typed job payload structs - mirror the job payload types for compile-time safety.
// These match the payload structs in internal/modules/site/jobs/*.go

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
	})
}

// dispatchAnalyzeLaravelFeatures dispatches the analyze Laravel features job with type-safe payload.
func (t *deploySiteTask) dispatchAnalyzeLaravelFeatures(cbCtx *taskrunner.CallbackContext) {
	t.dispatchJob(cbCtx, "site:analyze_laravel_features", analyzeLaravelFeaturesPayload{
		SiteID:   t.callback.SiteID,
		ServerID: t.callback.ServerID,
	})
}

// processNextQueuedDeployment finds and dispatches the next queued deployment
func (t *deploySiteTask) processNextQueuedDeployment(ctx context.Context, cbCtx *taskrunner.CallbackContext) {
	var nextDeployment models.Deployment
	err := cbCtx.DB.Where("site_id = ? AND status = ?", t.callback.SiteID, enums.DeploymentStatusQueued).
		Order("created_at ASC").
		First(&nextDeployment).Error

	if err != nil {
		return
	}

	// Update status to pending
	cbCtx.DB.Model(&nextDeployment).Update("status", enums.DeploymentStatusPending)

	// Get site to check zero downtime setting
	var site models.Site
	if err := cbCtx.DB.First(&site, "id = ?", t.callback.SiteID).Error; err != nil {
		return
	}

	jobType := "site:deploy"
	if site.ZeroDowntimeDeployment {
		jobType = "site:deploy_zero_downtime"
	}

	t.dispatchJob(cbCtx, jobType, map[string]string{
		"site_id":       t.callback.SiteID,
		"deployment_id": nextDeployment.ID,
	})
}

// restartQueueWorkers restarts queue workers after deployment
func (t *deploySiteTask) restartQueueWorkers(ctx context.Context, cbCtx *taskrunner.CallbackContext) {
	var queues []models.Queue
	cbCtx.DB.Where("site_id = ? AND installed_at IS NOT NULL", t.callback.SiteID).Find(&queues)

	for _, q := range queues {
		t.dispatchJob(cbCtx, "site:restart_queue", map[string]string{
			"site_id":  t.callback.SiteID,
			"queue_id": q.ID,
		})
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
	scriptBuilder.WriteString(templates.ShellDefaults())
	scriptBuilder.WriteString("\n\n")

	scriptBuilder.WriteString(templates.MustRender("deployment/shell_variables.sh", struct {
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

	if site.InstalledAt != nil && site.Type == enums.SiteTypeWordpress {
		scriptBuilder.WriteString("echo \"Wordpress already installed!\"\n\n")
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
	scriptBuilder.WriteString(templates.ShellDefaults())
	scriptBuilder.WriteString("\n\n")

	scriptBuilder.WriteString(templates.MustRender("deployment/shell_variables.sh", struct {
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
	scriptBuilder.WriteString(templates.MustRender("deployment/cleanup_old_releases.sh", struct {
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
	scriptBuilder.WriteString(templates.MustRender("deployment/link_shared_directories.sh", struct {
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
	scriptBuilder.WriteString(templates.MustRender("deployment/link_shared_files.sh", struct {
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
	scriptBuilder.WriteString(templates.MustRender("deployment/make_directories_writable.sh", struct {
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
	scriptBuilder.WriteString(templates.MustRender("deployment/make_deployment_current.sh", struct {
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
	return templates.MustRender("deployment/update_repository.sh", struct {
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
	case enums.SiteTypeLaravel:
		scriptBuilder.WriteString(templates.MustRender("deployment/prepare_fresh_installation/laravel.sh", struct {
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

	case enums.SiteTypeWordpress:
		scriptBuilder.WriteString(templates.MustRender("deployment/prepare_fresh_installation/wordpress.sh", struct {
			SitePath            string
			RepositoryDirectory string
			EnvVariables        map[string]string
		}{
			SitePath:            site.Path,
			RepositoryDirectory: repoDir,
			EnvVariables:        opts.EnvVariables,
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
	script := templates.MustRender("rollback_deployment.sh", struct {
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

