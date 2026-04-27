package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/models"
	dockerapptasks "github.com/kkz6/launch-go/internal/modules/dockerapp/tasks"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallApp = "dockerapp:uninstall"

// UninstallPayload carries the IDs needed to uninstall an app.
type UninstallPayload struct {
	AppID      string  `json:"app_id"`
	RemoveData bool    `json:"remove_data"`
	UserID     *string `json:"user_id,omitempty"`
}

// UninstallJob removes the application container (and optionally its
// named volumes) and deletes the row.
type UninstallJob struct {
	Deps    *JobDeps
	Payload UninstallPayload

	app    *models.App
	server *servermodels.Server
}

// NewUninstallJob constructs the job handler.
func NewUninstallJob(p UninstallPayload) pkgjobs.Handler {
	return &UninstallJob{Deps: deps, Payload: p}
}

// Handle runs the uninstall script and deletes the app row.
func (j *UninstallJob) Handle(ctx context.Context) error {
	app, err := j.Deps.Repos.App().FindByIDWithRelations(ctx, j.Payload.AppID)
	if err != nil {
		return fmt.Errorf("failed to find app: %w", err)
	}
	j.app = app

	server, err := j.Deps.GetServer(ctx, app.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.BroadcastAppEvent(j.server, "app.progress", j.app.ID, "uninstalling", fmt.Sprintf("Removing %s", j.app.Name))

	volumes := make([]dockerapptasks.Volume, 0, len(j.app.Volumes))
	for _, v := range j.app.Volumes {
		volumes = append(volumes, dockerapptasks.Volume{
			HostName:  types.DefaultVolumeName(j.app.Name, v.Name),
			MountPath: v.MountPath,
		})
	}

	task := dockerapptasks.Uninstall(dockerapptasks.UninstallOptions{
		AppName:    j.app.Name,
		Container:  j.app.Container(),
		Volumes:    volumes,
		RemoveData: j.Payload.RemoveData,
	})

	result, err := j.Deps.RunTask(j.server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to dispatch uninstall task: %w", err)
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("uninstall script failed: %s", result.GetOutput())
	}

	if err := j.Deps.Repos.App().Delete(ctx, j.app.ID); err != nil {
		return fmt.Errorf("failed to delete app row: %w", err)
	}

	j.Deps.BroadcastAppEvent(j.server, "app.progress", j.app.ID, "removed", fmt.Sprintf("%s removed", j.app.Name))
	return nil
}

// Failed records the error against the app.
func (j *UninstallJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("app_id", j.Payload.AppID).
		Msg("docker app uninstall failed")

	msg := err.Error()
	_ = j.Deps.Repos.App().Update(ctx, j.Payload.AppID, map[string]any{
		"status":     types.StatusFailed,
		"last_error": &msg,
	})
	if j.server != nil {
		j.Deps.BroadcastAppEvent(j.server, "app.progress", j.Payload.AppID, "failed", err.Error())
	}
}

// NewUninstallTask builds the asynq task.
func NewUninstallTask(appID string, removeData bool, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallApp, UninstallPayload{
		AppID:      appID,
		RemoveData: removeData,
		UserID:     userID,
	}, asynq.TaskID(pkgjobs.Dedup("uninstall_dockerapp", appID)))
}
