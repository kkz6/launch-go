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

const TypeInstallDockerService = "dockerservice:install"

// InstallPayload carries the IDs needed to install a docker service.
type InstallPayload struct {
	DockerServiceID string  `json:"docker_service_id"`
	UserID          *string `json:"user_id,omitempty"`
}

// InstallJob handles docker-service installation.
type InstallJob struct {
	Deps    *JobDeps
	Payload InstallPayload

	ms     *models.DockerService
	server *servermodels.Server
}

// NewInstallJob constructs the job handler.
func NewInstallJob(p InstallPayload) pkgjobs.Handler {
	return &InstallJob{Deps: deps, Payload: p}
}

// Handle runs the install task on the server, then marks the service running.
func (j *InstallJob) Handle(ctx context.Context) error {
	if err := j.load(ctx); err != nil {
		return err
	}

	if err := j.markStatus(ctx, dstypes.StatusInstalling); err != nil {
		return err
	}

	j.Deps.BroadcastDockerServiceEvent(j.server, "docker_service.progress", j.ms.ID, "installing", fmt.Sprintf("Starting %s", j.ms.Kind.Label()))

	username := ""
	if j.ms.Username != nil {
		username = *j.ms.Username
	}
	databaseName := ""
	if j.ms.DatabaseName != nil {
		databaseName = *j.ms.DatabaseName
	}

	task := dstasks.Install(dstasks.InstallOptions{
		Kind:         j.ms.Kind,
		Image:        j.ms.Image,
		Container:    j.ms.Container,
		Volume:       j.ms.Volume,
		Username:     username,
		Password:     j.ms.Password,
		DatabaseName: databaseName,
	})

	result, err := j.Deps.RunTask(j.server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to dispatch install task: %w", err)
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("install script failed: %s", result.GetOutput())
	}

	updates := map[string]any{
		"status":     dstypes.StatusRunning,
		"last_error": nil,
	}
	if result.TaskModel != nil {
		updates["task_id"] = result.TaskModel.ID
	}
	if err := j.Deps.Repos.DockerService().Update(ctx, j.ms.ID, updates); err != nil {
		return fmt.Errorf("failed to mark docker service running: %w", err)
	}

	j.Deps.BroadcastDockerServiceEvent(j.server, "docker_service.progress", j.ms.ID, "running", fmt.Sprintf("%s started", j.ms.Kind.Label()))
	return nil
}

// Failed marks the service as failed and records the error.
func (j *InstallJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("docker_service_id", j.Payload.DockerServiceID).
		Msg("docker service install failed")

	msg := err.Error()
	_ = j.Deps.Repos.DockerService().Update(ctx, j.Payload.DockerServiceID, map[string]any{
		"status":     dstypes.StatusFailed,
		"last_error": &msg,
	})

	if j.server != nil {
		j.Deps.BroadcastDockerServiceEvent(j.server, "docker_service.progress", j.Payload.DockerServiceID, "failed", err.Error())
	}
}

func (j *InstallJob) load(ctx context.Context) error {
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
	return nil
}

func (j *InstallJob) markStatus(ctx context.Context, status dstypes.Status) error {
	return j.Deps.Repos.DockerService().Update(ctx, j.ms.ID, map[string]any{"status": status})
}

// NewInstallTask builds the asynq task for install.
func NewInstallTask(dockerServiceID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallDockerService, InstallPayload{
		DockerServiceID: dockerServiceID,
		UserID:          userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_docker_service", dockerServiceID)))
}
