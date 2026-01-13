package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

type AddServiceJob struct {
	ServerJobBase
	jobs.InstallationTracker
	Payload AddServicePayload
}

func (j *AddServiceJob) Type() string {
	return TypeAddService
}

func (j *AddServiceJob) Handle(ctx context.Context) error {
	service, err := j.Repo().FindServiceByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	if err := j.Repo().UpdateServiceStatus(ctx, service.ID, enums.ServiceStatusInstalling); err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	software := enums.Software(j.Payload.Software)

	task := tasks.InstallSoftware(software)

	result, err := j.RunTaskOnServer(server, task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to install service: %s", result.GetOutput())
	}

	if err := j.Repo().UpdateServiceStatus(ctx, service.ID, enums.ServiceStatusRunning); err != nil {
		return fmt.Errorf("failed to update service status: %w", err)
	}

	j.LogInfo("Service installed successfully",
		"service_id", service.ID,
		"server_id", server.ID,
		"software", j.Payload.Software,
	)

	j.BroadcastServerEvent(server.ID, "service.installed", map[string]any{
		"service_id": service.ID,
		"server_id":  server.ID,
		"software":   j.Payload.Software,
	})

	return nil
}

func (j *AddServiceJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install service",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
	)

	_ = j.Repo().UpdateServiceStatus(ctx, j.Payload.ServiceID, enums.ServiceStatusFailed)
}
