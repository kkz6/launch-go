package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

const TypeEnableLaravelScheduler = "site:enable_laravel_scheduler"

// EnableLaravelSchedulerPayload holds data for enabling Laravel scheduler
type EnableLaravelSchedulerPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// EnableLaravelSchedulerJob enables the Laravel scheduler cron for a site
type EnableLaravelSchedulerJob struct {
	pkgjobs.BaseJob[*JobContext, EnableLaravelSchedulerPayload]
}

// NewEnableLaravelSchedulerJob creates a new EnableLaravelSchedulerJob
func NewEnableLaravelSchedulerJob(ctx *JobContext, payload EnableLaravelSchedulerPayload) *EnableLaravelSchedulerJob {
	return &EnableLaravelSchedulerJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the enable scheduler job
func (j *EnableLaravelSchedulerJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Check if site is Laravel type
	if site.Type != enums.SiteTypeLaravel {
		return fmt.Errorf("scheduler can only be enabled for Laravel sites")
	}

	// Check if scheduler is already enabled
	if site.HasEnabledFeature("scheduler") {
		j.Ctx.LogInfo("Scheduler already enabled", "site_id", site.ID)
		return nil
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Enabling Laravel scheduler",
		"site_id", site.ID,
		"server_id", server.ID,
	)

	// Build scheduler command
	command := j.buildSchedulerCommand(site)

	// Create cron record
	schedule := serverenums.CronEveryMinute
	cron := &servermodels.Cron{
		SiteID:     &site.ID,
		Expression: schedule.Expression(),
		Command:    basemodels.EncryptedString(command),
		User:       site.User,
		Frequency:  schedule.FrequencyName(),
		Hidden:     true, // Laravel scheduler crons are hidden
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

	// Update site's enabled_features
	now := time.Now()
	feature := models.EnabledFeature{
		Name:      "scheduler",
		CronID:    &cron.ID,
		EnabledAt: &now,
	}
	site.AddEnabledFeature(feature)
	site.RemovePendingFeature("scheduler")

	if err := j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Ctx.LogError(err, "Failed to update site enabled_features")
	}

	// Broadcast success
	j.Ctx.BroadcastServerEvent(server, "site.scheduler_enabled", map[string]interface{}{
		"site_id": site.ID,
		"cron_id": cron.ID,
	})

	j.Ctx.LogInfo("Laravel scheduler enabled successfully",
		"site_id", site.ID,
		"cron_id", cron.ID,
	)

	return nil
}

// buildSchedulerCommand builds the artisan schedule:run command
func (j *EnableLaravelSchedulerJob) buildSchedulerCommand(site *models.Site) string {
	return fmt.Sprintf("cd %s && %s artisan schedule:run >> /dev/null 2>&1",
		site.GetApplicationDirectory(), site.GetPhpBinary())
}

// dispatchInstallCron dispatches the server InstallCron job
func (j *EnableLaravelSchedulerJob) dispatchInstallCron(cronID, serverID string) error {
	task, err := serverjobs.NewInstallCronTask(serverID, cronID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Ctx.DispatchTask(task)
}

// Failed handles job failure
func (j *EnableLaravelSchedulerJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to enable Laravel scheduler",
		"site_id", j.Payload.SiteID,
	)

	// Remove from pending features
	site, findErr := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		site.RemovePendingFeature("scheduler")
		_ = j.Ctx.SiteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
			"pending_features": site.PendingFeatures,
		})
	}
}

// NewEnableLaravelSchedulerTask creates an enable scheduler task
func NewEnableLaravelSchedulerTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeEnableLaravelScheduler, EnableLaravelSchedulerPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
