package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/managedservice/models"
	mstasks "github.com/kkz6/launch-go/internal/modules/managedservice/tasks"
	mstypes "github.com/kkz6/launch-go/internal/modules/managedservice/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallManagedService = "managedservice:install"

// InstallPayload carries the IDs needed to install a managed service.
type InstallPayload struct {
	ManagedServiceID string  `json:"managed_service_id"`
	UserID           *string `json:"user_id,omitempty"`
}

// InstallJob handles managed-service installation.
type InstallJob struct {
	Deps    *JobDeps
	Payload InstallPayload

	ms     *models.ManagedService
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

	if err := j.markStatus(ctx, mstypes.StatusInstalling); err != nil {
		return err
	}

	j.Deps.BroadcastManagedServiceEvent(j.server, "managed_service.progress", j.ms.ID, "installing", fmt.Sprintf("Starting %s", j.ms.Kind.Label()))

	username := ""
	if j.ms.Username != nil {
		username = *j.ms.Username
	}
	databaseName := ""
	if j.ms.DatabaseName != nil {
		databaseName = *j.ms.DatabaseName
	}

	task := mstasks.Install(mstasks.InstallOptions{
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
		"status":     mstypes.StatusRunning,
		"last_error": nil,
	}
	if result.TaskModel != nil {
		updates["task_id"] = result.TaskModel.ID
	}
	if err := j.Deps.Repos.ManagedService().Update(ctx, j.ms.ID, updates); err != nil {
		return fmt.Errorf("failed to mark managed service running: %w", err)
	}

	j.Deps.BroadcastManagedServiceEvent(j.server, "managed_service.progress", j.ms.ID, "running", fmt.Sprintf("%s started", j.ms.Kind.Label()))
	return nil
}

// Failed marks the service as failed and records the error.
func (j *InstallJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("managed_service_id", j.Payload.ManagedServiceID).
		Msg("managed service install failed")

	msg := err.Error()
	_ = j.Deps.Repos.ManagedService().Update(ctx, j.Payload.ManagedServiceID, map[string]any{
		"status":     mstypes.StatusFailed,
		"last_error": &msg,
	})

	if j.server != nil {
		j.Deps.BroadcastManagedServiceEvent(j.server, "managed_service.progress", j.Payload.ManagedServiceID, "failed", err.Error())
	}
}

func (j *InstallJob) load(ctx context.Context) error {
	ms, err := j.Deps.Repos.ManagedService().FindByID(ctx, j.Payload.ManagedServiceID)
	if err != nil {
		return fmt.Errorf("failed to find managed service: %w", err)
	}
	j.ms = ms

	server, err := j.Deps.GetServer(ctx, ms.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server
	return nil
}

func (j *InstallJob) markStatus(ctx context.Context, status mstypes.Status) error {
	return j.Deps.Repos.ManagedService().Update(ctx, j.ms.ID, map[string]any{"status": status})
}

// NewInstallTask builds the asynq task for install.
func NewInstallTask(managedServiceID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallManagedService, InstallPayload{
		ManagedServiceID: managedServiceID,
		UserID:           userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_managed_service", managedServiceID)))
}
