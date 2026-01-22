package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeEnableLaravelHorizon = "site:enable_laravel_horizon"

// EnableLaravelHorizonPayload holds data for enabling Laravel Horizon
type EnableLaravelHorizonPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// EnableLaravelHorizonJob enables Laravel Horizon for a site
type EnableLaravelHorizonJob struct {
	Deps    *JobDeps
	Payload EnableLaravelHorizonPayload
	FeatureJobHelpers

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewEnableLaravelHorizonJob creates a new EnableLaravelHorizonJob
func NewEnableLaravelHorizonJob(p EnableLaravelHorizonPayload) pkgjobs.Handler {
	return &EnableLaravelHorizonJob{
		Deps:              deps,
		Payload:           p,
		FeatureJobHelpers: FeatureJobHelpers{Deps: deps},
	}
}

// Handle executes the enable Horizon job
func (j *EnableLaravelHorizonJob) Handle(ctx context.Context) error {
	result, err := j.LoadAndValidate(ctx, j.Payload.SiteID, j.Payload.ServerID, FeatureHorizon)
	if err != nil {
		return err
	}
	if result.AlreadyEnabled {
		return nil
	}

	site, server := result.Site, result.Server
	j.site = site
	j.server = server

	// Horizon and queue workers conflict
	if site.HasEnabledFeature(FeatureQueue) {
		return fmt.Errorf("cannot enable Horizon while queue workers are enabled - disable queue workers first")
	}

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("server_id", server.ID).Msg("Enabling Laravel Horizon")

	userID := j.GetUserID(j.Payload.UserID, site)
	command := fmt.Sprintf("%s %s/artisan horizon", site.GetPhpBinary(), site.GetApplicationDirectory())

	queue, err := j.CreateQueueRecord(ctx, site, server, command, userID)
	if err != nil {
		return err
	}

	if err := j.DispatchInstallQueue(ctx, queue.ID, site.ID, j.Payload.UserID); err != nil {
		return err
	}

	j.EnableFeature(ctx, site, FeatureHorizon, &queue.ID, nil)
	j.BroadcastFeatureEnabled(server, FeatureHorizon, site.ID, queue.ID)
	j.Deps.Logger.Info().Str("site_id", site.ID).Str("queue_id", queue.ID).Msg("Laravel Horizon enabled successfully")

	return nil
}

// Failed handles job failure
func (j *EnableLaravelHorizonJob) Failed(ctx context.Context, err error) {
	j.HandleFailure(ctx, err, j.Payload.SiteID, FeatureHorizon, "Failed to enable Laravel Horizon")
}

// NewEnableLaravelHorizonTask creates an enable Horizon task
func NewEnableLaravelHorizonTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeEnableLaravelHorizon, EnableLaravelHorizonPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
