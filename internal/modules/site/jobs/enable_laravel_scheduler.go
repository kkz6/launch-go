package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	Deps    *JobDeps
	Payload EnableLaravelSchedulerPayload
	FeatureJobHelpers

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewEnableLaravelSchedulerJob creates a new EnableLaravelSchedulerJob
func NewEnableLaravelSchedulerJob(p EnableLaravelSchedulerPayload) pkgjobs.Handler {
	return &EnableLaravelSchedulerJob{
		Deps:              deps,
		Payload:           p,
		FeatureJobHelpers: FeatureJobHelpers{Deps: deps},
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
	j.site = site
	j.server = server

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("server_id", server.ID).Msg("Enabling Laravel scheduler")

	cron, err := j.createSchedulerCron(ctx, site, server)
	if err != nil {
		return err
	}

	if err := j.dispatchInstallCron(ctx, cron.ID, server.ID); err != nil {
		return err
	}

	j.EnableFeature(ctx, site, FeatureScheduler, nil, &cron.ID)
	j.BroadcastSchedulerEnabled(server, site.ID, cron.ID)
	j.Deps.Logger.Info().Str("site_id", site.ID).Str("cron_id", cron.ID).Msg("Laravel scheduler enabled successfully")

	return nil
}

// createSchedulerCron creates the cron record for Laravel scheduler
func (j *EnableLaravelSchedulerJob) createSchedulerCron(ctx context.Context, site *models.Site, server *servermodels.Server) (*servermodels.Cron, error) {
	command := fmt.Sprintf("cd %s && %s artisan schedule:run >> /dev/null 2>&1",
		site.GetApplicationDirectory(), site.GetPhpBinary())

	schedule := servertypes.CronEveryMinute
	cron := &servermodels.Cron{
		SiteID:     &site.ID,
		Expression: schedule.Expression(),
		Command:    dbtype.EncryptedString(command),
		User:       site.User,
		Frequency:  schedule.FrequencyName(),
		Hidden:     true,
	}
	cron.ServerID = server.ID

	if err := j.Deps.ServerRepos.Cron().Create(ctx, cron); err != nil {
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

	if err := j.Deps.DispatchTask(task); err != nil {
		_ = j.Deps.ServerRepos.Cron().Delete(ctx, cronID)
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
	return pkgjobs.Task(TypeEnableLaravelScheduler, EnableLaravelSchedulerPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
