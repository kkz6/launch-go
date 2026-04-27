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

const TypeUninstallManagedService = "managedservice:uninstall"

// UninstallPayload carries the IDs needed to uninstall a managed service.
type UninstallPayload struct {
	ManagedServiceID string  `json:"managed_service_id"`
	RemoveData       bool    `json:"remove_data"`
	UserID           *string `json:"user_id,omitempty"`
}

// UninstallJob removes a managed service container (and optionally its volume).
type UninstallJob struct {
	Deps    *JobDeps
	Payload UninstallPayload

	ms     *models.ManagedService
	server *servermodels.Server
}

// NewUninstallJob constructs the job handler.
func NewUninstallJob(p UninstallPayload) pkgjobs.Handler {
	return &UninstallJob{Deps: deps, Payload: p}
}

// Handle runs the uninstall script and deletes the row.
func (j *UninstallJob) Handle(ctx context.Context) error {
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

	j.Deps.BroadcastManagedServiceEvent(j.server, "managed_service.progress", j.ms.ID, "uninstalling", fmt.Sprintf("Removing %s", j.ms.Kind.Label()))

	task := mstasks.Uninstall(mstasks.UninstallOptions{
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

	if err := j.Deps.Repos.ManagedService().Delete(ctx, j.ms.ID); err != nil {
		return fmt.Errorf("failed to delete managed service row: %w", err)
	}

	j.Deps.BroadcastManagedServiceEvent(j.server, "managed_service.progress", j.ms.ID, "removed", fmt.Sprintf("%s removed", j.ms.Kind.Label()))
	return nil
}

// Failed marks the service as failed and records the error.
func (j *UninstallJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("managed_service_id", j.Payload.ManagedServiceID).
		Msg("managed service uninstall failed")

	msg := err.Error()
	_ = j.Deps.Repos.ManagedService().Update(ctx, j.Payload.ManagedServiceID, map[string]any{
		"status":     mstypes.StatusFailed,
		"last_error": &msg,
	})

	if j.server != nil {
		j.Deps.BroadcastManagedServiceEvent(j.server, "managed_service.progress", j.Payload.ManagedServiceID, "failed", err.Error())
	}
}

// NewUninstallTask builds the asynq task for uninstall.
func NewUninstallTask(managedServiceID string, removeData bool, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallManagedService, UninstallPayload{
		ManagedServiceID: managedServiceID,
		RemoveData:       removeData,
		UserID:           userID,
	}, asynq.TaskID(pkgjobs.Dedup("uninstall_managed_service", managedServiceID)))
}
