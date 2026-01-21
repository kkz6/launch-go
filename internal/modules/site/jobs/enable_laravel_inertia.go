package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeEnableLaravelInertia = "site:enable_laravel_inertia"

// EnableLaravelInertiaPayload holds data for enabling Laravel Inertia SSR
type EnableLaravelInertiaPayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// EnableLaravelInertiaJob enables Inertia SSR for a Laravel site
type EnableLaravelInertiaJob struct {
	pkgjobs.BaseJob[*JobContext, EnableLaravelInertiaPayload]
	FeatureJobHelpers
}

// NewEnableLaravelInertiaJob creates a new EnableLaravelInertiaJob
func NewEnableLaravelInertiaJob(ctx *JobContext, payload EnableLaravelInertiaPayload) *EnableLaravelInertiaJob {
	return &EnableLaravelInertiaJob{
		BaseJob:           pkgjobs.NewBaseJob(ctx, payload),
		FeatureJobHelpers: FeatureJobHelpers{Ctx: ctx},
	}
}

// Handle executes the enable Inertia job
func (j *EnableLaravelInertiaJob) Handle(ctx context.Context) error {
	result, err := j.LoadAndValidate(ctx, j.Payload.SiteID, j.Payload.ServerID, FeatureInertia)
	if err != nil {
		return err
	}
	if result.AlreadyEnabled {
		return nil
	}

	site, server := result.Site, result.Server
	j.BaseJob.Ctx.LogInfo("Enabling Laravel Inertia SSR", "site_id", site.ID, "server_id", server.ID)

	userID := j.GetUserID(j.Payload.UserID, site)
	command := fmt.Sprintf("node %s/bootstrap/ssr/ssr.js", site.GetApplicationDirectory())

	queue, err := j.CreateQueueRecord(ctx, site, server, command, userID)
	if err != nil {
		return err
	}

	if err := j.DispatchInstallQueue(ctx, queue.ID, site.ID, j.Payload.UserID); err != nil {
		return err
	}

	j.EnableFeature(ctx, site, FeatureInertia, &queue.ID, nil)
	j.BroadcastFeatureEnabled(server, FeatureInertia, site.ID, queue.ID)
	j.BaseJob.Ctx.LogInfo("Laravel Inertia SSR enabled successfully", "site_id", site.ID, "queue_id", queue.ID)

	return nil
}

// Failed handles job failure
func (j *EnableLaravelInertiaJob) Failed(ctx context.Context, err error) {
	j.HandleFailure(ctx, err, j.Payload.SiteID, FeatureInertia, "Failed to enable Laravel Inertia SSR")
}

// NewEnableLaravelInertiaTask creates an enable Inertia task
func NewEnableLaravelInertiaTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeEnableLaravelInertia, EnableLaravelInertiaPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
