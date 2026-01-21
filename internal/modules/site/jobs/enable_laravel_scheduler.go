package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
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
	FeatureJobHelpers
}

// NewEnableLaravelSchedulerJob creates a new EnableLaravelSchedulerJob
func NewEnableLaravelSchedulerJob(ctx *JobContext, payload EnableLaravelSchedulerPayload) *EnableLaravelSchedulerJob {
	return &EnableLaravelSchedulerJob{
		BaseJob:           pkgjobs.NewBaseJob(ctx, payload),
		FeatureJobHelpers: FeatureJobHelpers{Ctx: ctx},
	}
}

// Handle executes the enable scheduler job
func (j *EnableLaravelSchedulerJob) Handle(ctx context.Context) error {
	result, err := j.LoadAndValidate(ctx, j.Payload.SiteID, j.Payload.ServerID, FeatureScheduler)
	if err != nil {
		return err
	}
	if result.AlreadyEnabled {
		return nil
	}

	site, server := result.Site, result.Server
	j.BaseJob.Ctx.LogInfo("Enabling Laravel scheduler", "site_id", site.ID, "server_id", server.ID)

	cron, err := j.createSchedulerCron(ctx, site, server)
	if err != nil {
		return err
	}

	if err := j.dispatchInstallCron(ctx, cron.ID, server.ID); err != nil {
		return err
	}

	j.EnableFeature(ctx, site, FeatureScheduler, nil, &cron.ID)
	j.BroadcastSchedulerEnabled(server, site.ID, cron.ID)
	j.BaseJob.Ctx.LogInfo("Laravel scheduler enabled successfully", "site_id", site.ID, "cron_id", cron.ID)

	return nil
}

// createSchedulerCron creates the cron record for Laravel scheduler
func (j *EnableLaravelSchedulerJob) createSchedulerCron(ctx context.Context, site *models.Site, server *servermodels.Server) (*servermodels.Cron, error) {
	command := fmt.Sprintf("cd %s && %s artisan schedule:run >> /dev/null 2>&1",
		site.GetApplicationDirectory(), site.GetPhpBinary())

	schedule := serverenums.CronEveryMinute
	cron := &servermodels.Cron{
		SiteID:     &site.ID,
		Expression: schedule.Expression(),
		Command:    basemodels.EncryptedString(command),
		User:       site.User,
		Frequency:  schedule.FrequencyName(),
		Hidden:     true,
	}
	cron.ServerID = server.ID

	if err := j.BaseJob.Ctx.ServerRepos.Cron().Create(ctx, cron); err != nil {
		return nil, fmt.Errorf("failed to create cron: %w", err)
	}

	return cron, nil
}

// dispatchInstallCron dispatches the server InstallCron job
func (j *EnableLaravelSchedulerJob) dispatchInstallCron(ctx context.Context, cronID, serverID string) error {
	task, err := serverjobs.NewInstallCronTask(serverID, cronID, j.Payload.UserID)
	if err != nil {
		return err
	}

	if err := j.BaseJob.Ctx.DispatchTask(task); err != nil {
		_ = j.BaseJob.Ctx.ServerRepos.Cron().Delete(ctx, cronID)
		return fmt.Errorf("failed to dispatch install cron job: %w", err)
	}

	return nil
}

// Failed handles job failure
func (j *EnableLaravelSchedulerJob) Failed(ctx context.Context, err error) {
	j.HandleFailure(ctx, err, j.Payload.SiteID, FeatureScheduler, "Failed to enable Laravel scheduler")
}

// NewEnableLaravelSchedulerTask creates an enable scheduler task
func NewEnableLaravelSchedulerTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeEnableLaravelScheduler, EnableLaravelSchedulerPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
