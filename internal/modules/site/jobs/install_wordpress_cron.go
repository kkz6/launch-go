package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
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
	Deps    *JobDeps
	Payload InstallWordpressCronPayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewInstallWordpressCronJob creates a new InstallWordpressCronJob
func NewInstallWordpressCronJob(p InstallWordpressCronPayload) pkgjobs.Handler {
	return &InstallWordpressCronJob{Deps: deps, Payload: p}
}

// Handle executes the install WordPress cron job
func (j *InstallWordpressCronJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Check if site is WordPress type
	if site.Type != sitetypes.SiteTypeWordpress {
		j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Site is not WordPress, skipping cron installation")
		return nil
	}

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("server_id", server.ID).Msg("Installing WordPress cron")

	// Build WordPress cron command
	// WordPress cron runs wp-cron.php every minute
	command := j.buildWpCronCommand(site)

	// Create cron record
	schedule := servertypes.CronEveryMinute
	cron := &servermodels.Cron{
		SiteID:     &site.ID,
		Expression: schedule.Expression(),
		Command:    basemodels.EncryptedString(command),
		User:       site.User,
		Frequency:  schedule.FrequencyName(),
		Hidden:     true, // WordPress crons are hidden system crons
	}
	cron.ServerID = server.ID

	if err := j.Deps.ServerRepos.Cron().Create(ctx, cron); err != nil {
		return fmt.Errorf("failed to create cron: %w", err)
	}

	// Dispatch InstallCron job
	if err := j.dispatchInstallCron(cron.ID, server.ID); err != nil {
		// Cleanup the cron record if dispatch fails
		_ = j.Deps.ServerRepos.Cron().Delete(ctx, cron.ID)
		return fmt.Errorf("failed to dispatch install cron job: %w", err)
	}

	// Broadcast success
	j.Deps.BroadcastServerEvent(server, "site.wordpress_cron_installed", map[string]interface{}{
		"site_id": site.ID,
		"cron_id": cron.ID,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("cron_id", cron.ID).Msg("WordPress cron installed successfully")

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
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *InstallWordpressCronJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Failed to install WordPress cron")
}

// NewInstallWordpressCronTask creates an install WordPress cron task
func NewInstallWordpressCronTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallWordpressCron, InstallWordpressCronPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
