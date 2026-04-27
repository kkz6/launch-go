package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/dockerservice/models"
	dstasks "github.com/kkz6/launch-go/internal/modules/dockerservice/tasks"
	dstypes "github.com/kkz6/launch-go/internal/modules/dockerservice/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallDockerService = "dockerservice:uninstall"

// UninstallPayload carries the IDs needed to uninstall a docker service.
type UninstallPayload struct {
	DockerServiceID string  `json:"docker_service_id"`
	RemoveData      bool    `json:"remove_data"`
	UserID          *string `json:"user_id,omitempty"`
}

// UninstallJob removes a docker service container (and optionally its volume).
type UninstallJob struct {
	Deps    *JobDeps
	Payload UninstallPayload

	ms     *models.DockerService
	server *servermodels.Server
}

// NewUninstallJob constructs the job handler.
func NewUninstallJob(p UninstallPayload) pkgjobs.Handler {
	return &UninstallJob{Deps: deps, Payload: p}
}

// Handle runs the uninstall script and deletes the row.
func (j *UninstallJob) Handle(ctx context.Context) error {
	ms, err := j.Deps.Repos.DockerService().FindByID(ctx, j.Payload.DockerServiceID)
	if err != nil {
		return fmt.Errorf("failed to find docker service: %w", err)
	}
	j.ms = ms

	server, err := j.Deps.GetServer(ctx, ms.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.BroadcastDockerServiceEvent(j.server, "docker_service.progress", j.ms.ID, "uninstalling", fmt.Sprintf("Removing %s", j.ms.Kind.Label()))

	task := dstasks.Uninstall(dstasks.UninstallOptions{
		Kind:       j.ms.Kind,
		Container:  j.ms.Container,
		Volume:     j.ms.Volume,
		RemoveData: j.Payload.RemoveData,
	})

	result, err := j.Deps.RunTask(j.server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to dispatch uninstall task: %w", err)
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("uninstall script failed: %s", result.GetOutput())
	}

	if err := j.Deps.Repos.DockerService().Delete(ctx, j.ms.ID); err != nil {
		return fmt.Errorf("failed to delete docker service row: %w", err)
	}

	j.Deps.BroadcastDockerServiceEvent(j.server, "docker_service.progress", j.ms.ID, "removed", fmt.Sprintf("%s removed", j.ms.Kind.Label()))
	return nil
}

// Failed marks the service as failed and records the error.
func (j *UninstallJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("docker_service_id", j.Payload.DockerServiceID).
		Msg("docker service uninstall failed")

	msg := err.Error()
	_ = j.Deps.Repos.DockerService().Update(ctx, j.Payload.DockerServiceID, map[string]any{
		"status":     dstypes.StatusFailed,
		"last_error": &msg,
	})

	if j.server != nil {
		j.Deps.BroadcastDockerServiceEvent(j.server, "docker_service.progress", j.Payload.DockerServiceID, "failed", err.Error())
	}
}

// NewUninstallTask builds the asynq task for uninstall.
func NewUninstallTask(dockerServiceID string, removeData bool, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallDockerService, UninstallPayload{
		DockerServiceID: dockerServiceID,
		RemoveData:      removeData,
		UserID:          userID,
	}, asynq.TaskID(pkgjobs.Dedup("uninstall_docker_service", dockerServiceID)))
}
