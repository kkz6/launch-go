package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

const TypeInstallWordpressCron = "site:install_wordpress_cron"

// InstallWordpressCronPayload holds data for WordPress cron installation
type InstallWordpressCronPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallWordpressCronJob installs the WordPress cron job for a site
type InstallWordpressCronJob struct {
	pkgjobs.BaseJob[*JobContext, InstallWordpressCronPayload]
}

// NewInstallWordpressCronJob creates a new InstallWordpressCronJob
func NewInstallWordpressCronJob(ctx *JobContext, payload InstallWordpressCronPayload) *InstallWordpressCronJob {
	return &InstallWordpressCronJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the install WordPress cron job
func (j *InstallWordpressCronJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if site is WordPress type
	if site.Type != enums.SiteTypeWordpress {
		j.Ctx.LogInfo("Site is not WordPress, skipping cron installation", "site_id", site.ID)
		return nil
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Installing WordPress cron",
		"site_id", site.ID,
		"server_id", server.ID,
	)

	// Build WordPress cron command
	// WordPress cron runs wp-cron.php every minute
	command := j.buildWpCronCommand(site)

	// Create cron record
	cron := &servermodels.Cron{
		SiteID:     &site.ID,
		Expression: "* * * * *",
		Command:    basemodels.EncryptedString(command),
		User:       site.User,
		Frequency:  "every_minute",
		Hidden:     true, // WordPress crons are hidden system crons
	}
	cron.ServerID = server.ID

	if err := j.Ctx.ServerRepos.Cron().Create(ctx, cron); err != nil {
		return fmt.Errorf("failed to create cron: %w", err)
	}

	// Dispatch InstallCron job
	if err := j.dispatchInstallCron(cron.ID, server.ID); err != nil {
		// Cleanup the cron record if dispatch fails
		_ = j.Ctx.ServerRepos.Cron().Delete(ctx, cron.ID)
		return fmt.Errorf("failed to dispatch install cron job: %w", err)
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "site.wordpress_cron_installed", map[string]interface{}{
		"site_id": site.ID,
		"cron_id": cron.ID,
	})

	j.Ctx.LogInfo("WordPress cron installed successfully",
		"site_id", site.ID,
		"cron_id", cron.ID,
	)

	return nil
}

// buildWpCronCommand builds the WordPress cron command
func (j *InstallWordpressCronJob) buildWpCronCommand(site interface{ GetWebDirectory() string }) string {
	webDir := site.GetWebDirectory()
	// Use wget or curl to trigger wp-cron.php
	// We'll use wget as it's commonly available
	return fmt.Sprintf("wget -q -O - %s/wp-cron.php?doing_wp_cron >/dev/null 2>&1", webDir)
}

// dispatchInstallCron dispatches the server InstallCron job
func (j *InstallWordpressCronJob) dispatchInstallCron(cronID, serverID string) error {
	task, err := serverjobs.NewInstallCronTask(serverID, cronID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Ctx.DispatchTask(task)
}

// Failed handles job failure
func (j *InstallWordpressCronJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to install WordPress cron",
		"site_id", j.Payload.SiteID,
	)
}

// NewInstallWordpressCronTask creates an install WordPress cron task
func NewInstallWordpressCronTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallWordpressCron, InstallWordpressCronPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
