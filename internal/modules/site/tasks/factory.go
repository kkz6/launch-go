package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// SiteTaskType represents the type of site task operation
type SiteTaskType string

// Task operation constants (site-agnostic)
const (
	TaskDeploy          SiteTaskType = "deploy"
	TaskUpdateCaddyfile SiteTaskType = "update_caddyfile"
	TaskRemoveCaddyfile SiteTaskType = "remove_caddyfile"
	TaskDeleteSiteFiles SiteTaskType = "delete_site_files"
	TaskRestartQueues   SiteTaskType = "restart_queues"
	TaskRunCommand      SiteTaskType = "run_command"
	TaskCheckDaemon     SiteTaskType = "check_daemon"
	TaskAnalyzeFeatures SiteTaskType = "analyze_features"
)

// Factory provides site-aware task creation
type Factory struct {
	site *models.Site
}

// NewFactory creates a new site task factory
func NewFactory(site *models.Site) *Factory {
	return &Factory{site: site}
}

// Site returns the underlying site model
func (f *Factory) Site() *models.Site {
	return f.site
}

// IsLaravel returns true if the site is a Laravel application
func (f *Factory) IsLaravel() bool {
	return f.site.Type == sitetypes.SiteTypeLaravel
}

// IsWordpress returns true if the site is a WordPress application
func (f *Factory) IsWordpress() bool {
	return f.site.Type == sitetypes.SiteTypeWordpress
}

// IsStatic returns true if the site is a static site
func (f *Factory) IsStatic() bool {
	return f.site.Type == sitetypes.SiteTypeStatic
}

// IsPhpMyAdmin returns true if the site is a phpMyAdmin application
func (f *Factory) IsPhpMyAdmin() bool {
	return f.site.Type == sitetypes.SiteTypePhpMyAdmin
}

// TaskType returns the full task type string for a given operation based on the site type
func (f *Factory) TaskType(taskType SiteTaskType) string {
	return fmt.Sprintf("site:%s_%s", f.site.Type, taskType)
}

// UpdateCaddyfile creates a task to update the site's Caddyfile
func (f *Factory) UpdateCaddyfile(content string) *taskrunner.BaseTask {
	return UpdateCaddyfile(UpdateCaddyfileConfig{
		CaddyfilePath:    fmt.Sprintf("%s/Caddyfile", f.site.Path),
		CaddyfileContent: content,
	})
}

// RemoveCaddyfile creates a task to remove the site's Caddyfile
func (f *Factory) RemoveCaddyfile() *taskrunner.BaseTask {
	return RemoveCaddyfile(RemoveCaddyfileConfig{
		CaddyfilePath: fmt.Sprintf("%s/Caddyfile", f.site.Path),
	})
}

// PrettifyCaddyfile creates a task to format the site's Caddyfile
func (f *Factory) PrettifyCaddyfile() *taskrunner.BaseTask {
	return PrettifyCaddyfile(PrettifyCaddyfileConfig{
		CaddyfilePath: fmt.Sprintf("%s/Caddyfile", f.site.Path),
	})
}

// DeleteFiles creates a task to delete all site files
func (f *Factory) DeleteFiles() *taskrunner.BaseTask {
	return DeleteSiteFiles(DeleteSiteFilesConfig{
		SitePath: f.site.Path,
	})
}

// RunCommand creates a task to run a command in the site's directory
func (f *Factory) RunCommand(command string) *taskrunner.BaseTask {
	return RunCommand(RunCommandConfig{
		Command: command,
		Site:    f.site,
	})
}

// RunCommandInDirectory creates a task to run a command in a specific directory
func (f *Factory) RunCommandInDirectory(command string) *taskrunner.BaseTask {
	return RunCommandForSite(RunCommandForSiteConfig{
		ApplicationDirectory: f.site.GetApplicationDirectory(),
		Command:              command,
	})
}

// CheckDaemonStatus creates a task to check supervisor daemon status
func (f *Factory) CheckDaemonStatus() *taskrunner.BaseTask {
	return CheckDaemonStatus()
}

// RestartQueue creates a task to restart a specific queue
func (f *Factory) RestartQueue(programName string) *taskrunner.BaseTask {
	return RestartQueue(programName)
}

// RestartAllQueues creates a task to restart all queues for the site
func (f *Factory) RestartAllQueues(queueIDs []string) *taskrunner.BaseTask {
	return RestartAllQueues(queueIDs)
}

// UploadQueueConfig creates a task to upload a queue supervisor config
func (f *Factory) UploadQueueConfig(queue *models.Queue, serverUsername string) *taskrunner.BaseTask {
	return UploadQueueConfig(UploadQueueConfigParams{
		Path:         queue.GetPath(),
		Contents:     BuildQueueSupervisorConfig(queue, serverUsername),
		LogPath:      queue.GetLogPath(),
		ErrorLogPath: queue.GetErrorLogPath(),
		User:         serverUsername,
		WorkingDir:   f.site.GetApplicationDirectory(),
	})
}

// DeleteQueueConfig creates a task to delete a queue config
func (f *Factory) DeleteQueueConfig(configPath, programName string) *taskrunner.BaseTask {
	return DeleteQueueConfig(configPath, programName)
}

// AnalyzeFeatures creates a task to read composer.json for Laravel feature detection
func (f *Factory) AnalyzeFeatures() *taskrunner.BaseTask {
	if !f.IsLaravel() {
		return nil
	}
	return ReadComposerJSON(f.site.GetApplicationDirectory())
}

// GetPhpBinary returns the PHP binary path for the site
func (f *Factory) GetPhpBinary() string {
	return f.site.GetPhpBinary()
}

// GetApplicationDirectory returns the application directory for the site
func (f *Factory) GetApplicationDirectory() string {
	return f.site.GetApplicationDirectory()
}

// GetLogsDirectory returns the logs directory for the site
func (f *Factory) GetLogsDirectory() string {
	return f.site.GetLogsDirectory()
}

// GetWebDirectory returns the web directory for the site
func (f *Factory) GetWebDirectory() string {
	return f.site.GetWebDirectory()
}
