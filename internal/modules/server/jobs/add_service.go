package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeAddService = "server:add_service"

type AddServicePayload struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
	Software  string `json:"software"`
}

type AddServiceJob struct {
	ctx     *JobContext
	Payload AddServicePayload
}

func (j *AddServiceJob) Handle(ctx context.Context) error {
	service, err := j.ctx.Repo.FindServiceByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	server, err := j.ctx.Repo.FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	if err := j.ctx.Repo.UpdateServiceStatus(ctx, service.ID, enums.ServiceStatusInstalling); err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	software := enums.Software(j.Payload.Software)

	task := tasks.InstallSoftware(software)

	result, err := j.ctx.ForServer(server).RunTask(task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to install service: %s", result.GetOutput())
	}

	if err := j.ctx.Repo.UpdateServiceStatus(ctx, service.ID, enums.ServiceStatusRunning); err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	j.ctx.LogInfo("Service installed successfully",
		"service_id", service.ID,
		"server_id", server.ID,
		"software", j.Payload.Software,
	)

	j.ctx.BroadcastToServer(server.ID, "service.installed", map[string]any{
		"service_id": service.ID,
		"server_id":  server.ID,
		"software":   j.Payload.Software,
	})

	return nil
}

func (j *AddServiceJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to install service",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
	)

	_ = j.ctx.Repo.UpdateServiceStatus(ctx, j.Payload.ServiceID, enums.ServiceStatusFailed)
}

func NewAddServiceJob(ctx *JobContext, payload AddServicePayload) *AddServiceJob {
	return &AddServiceJob{
		ctx:     ctx,
		Payload: payload,
	}
}

func NewAddServiceTask(serverID, serviceID, software string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeAddService, AddServicePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Software:  software,
	})
}
