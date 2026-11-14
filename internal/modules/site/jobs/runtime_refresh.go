package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRefreshRuntime = "site:refresh_runtime"

type RefreshRuntimePayload struct {
	SiteID   string `json:"site_id"`
	ServerID string `json:"server_id"`
	TeamID   string `json:"team_id"`
}

type RefreshRuntimeJob struct {
	Deps    *JobDeps
	Payload RefreshRuntimePayload
	taskID  string
}

func NewRefreshRuntimeJob(payload RefreshRuntimePayload) pkgjobs.Handler {
	return &RefreshRuntimeJob{Deps: deps, Payload: payload}
}

func NewRefreshRuntimeTask(siteID, serverID, teamID string, opts ...asynq.Option) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRefreshRuntime, RefreshRuntimePayload{SiteID: siteID, ServerID: serverID, TeamID: teamID}, opts...)
}

func (j *RefreshRuntimeJob) Handle(ctx context.Context) error {
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return err
	}
	if site.TeamID != j.Payload.TeamID || site.ServerID != j.Payload.ServerID {
		return fmt.Errorf("runtime refresh scope changed")
	}
	if !site.ZeroDowntimeDeployment || !(site.Type.IsPHP() || site.Type == sitetypes.SiteTypeGeneric) || site.PendingPhpVersion != nil {
		return fmt.Errorf("site is not ready for PHP runtime refresh")
	}
	active, err := j.Deps.Repos.Deployment().FindActiveBySite(ctx, site.ID)
	if err != nil {
		return err
	}
	if active != nil {
		return fmt.Errorf("a deployment is already in progress")
	}
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return err
	}
	if server.TeamID != site.TeamID {
		return fmt.Errorf("runtime refresh server scope changed")
	}
	task, err := tasks.RefreshRuntime(tasks.RuntimeActivationConfig{SitePath: site.Path, SiteType: site.Type, PhpVersion: site.PhpVersion})
	if err != nil {
		return err
	}
	result, err := j.Deps.RunTask(server, task).AsUser(site.User).ForSite(site.ID).
		OnTaskCreated(func(id string) {
			j.taskID = id
			j.broadcast("deployment.runtime.started", "Refreshing PHP runtime")
		}).Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("refresh PHP runtime: %w", err)
	}
	if result == nil || result.Error != nil || !result.IsSuccessful() {
		return fmt.Errorf("PHP runtime refresh failed; inspect the task log")
	}
	// Use the established worker restart policy. This never replays a code
	// switch, deployment hook, build, or database migration.
	if site.AutoRestartQueue {
		if err := j.queueWorkerRestart(site.ID); err != nil {
			j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("PHP refreshed but worker restart could not be queued")
			j.Deps.BroadcastToTeam(j.Payload.TeamID, "deployment.runtime.completed", map[string]any{
				"site_id": site.ID, "server_id": server.ID, "task_id": j.taskID,
				"post_actions_pending": true,
				"message":              "PHP runtime refreshed, but worker restart could not be queued. Retry runtime refresh to queue the workers.",
			})
			return nil
		}
	}
	j.broadcast("deployment.runtime.completed", "PHP runtime refreshed")
	return nil
}

func (j *RefreshRuntimeJob) queueWorkerRestart(siteID string) error {
	task, err := NewRestartAllSiteQueuesTask(siteID, nil)
	if err != nil {
		return err
	}
	if j.Deps.Queue == nil {
		return fmt.Errorf("worker queue is unavailable")
	}
	_, err = j.Deps.Queue.Enqueue(task)
	return err
}

func (j *RefreshRuntimeJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("PHP runtime refresh failed")
	j.broadcast("deployment.runtime.failed", "PHP runtime refresh failed. Check the task log before retrying.")
}

func (j *RefreshRuntimeJob) broadcast(event, message string) {
	j.Deps.BroadcastToTeam(j.Payload.TeamID, event, map[string]any{
		"site_id": j.Payload.SiteID, "server_id": j.Payload.ServerID,
		"task_id": j.taskID, "message": message,
	})
}
