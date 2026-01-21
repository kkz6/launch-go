package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSetDefaultPhp = "server:set_default_php"

type SetDefaultPhpPayload struct {
	ServerID  string  `json:"server_id"`
	ServiceID string  `json:"service_id"`
	Version   string  `json:"version"`
	UserID    *string `json:"user_id,omitempty"`
}

// SetDefaultPhpJob sets the default PHP version on a server
type SetDefaultPhpJob struct {
	pkgjobs.BaseJob[*JobContext, SetDefaultPhpPayload]
}

func (j *SetDefaultPhpJob) Handle(ctx context.Context) error {
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	service, err := j.Ctx.Repos().Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	j.Ctx.LogInfo("Setting default PHP version",
		"server_id", server.ID,
		"version", j.Payload.Version,
	)

	// Run the update alternatives task
	task := tasks.UpdateAlternatives(j.Payload.Version)

	result, err := j.Ctx.ForServer(server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to set default PHP: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to set default PHP: %s", result.GetOutput())
	}

	// Update the service record to mark it as default
	// First, unset any existing default
	if err := j.Ctx.Repos().Service().UnsetDefaultPhp(ctx, j.Payload.ServerID); err != nil {
		j.Ctx.LogError(err, "Failed to unset existing default PHP")
	}

	// Set the new default
	if err := j.Ctx.Repos().Service().SetDefault(ctx, j.Payload.ServiceID, true); err != nil {
		return fmt.Errorf("failed to update service default status: %w", err)
	}

	j.Ctx.LogInfo("Default PHP version set successfully",
		"server_id", server.ID,
		"service_id", service.ID,
		"version", j.Payload.Version,
	)

	j.Ctx.BroadcastServerEvent(server, "php.default_changed", map[string]any{
		"server_id":  server.ID,
		"service_id": service.ID,
		"version":    j.Payload.Version,
	})

	return nil
}

func (j *SetDefaultPhpJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to set default PHP version",
		"server_id", j.Payload.ServerID,
		"version", j.Payload.Version,
	)
}

func NewSetDefaultPhpJob(ctx *JobContext, payload SetDefaultPhpPayload) *SetDefaultPhpJob {
	return &SetDefaultPhpJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

func NewSetDefaultPhpTask(serverID, serviceID, version string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeSetDefaultPhp, SetDefaultPhpPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Version:   version,
		UserID:    userID,
	})
}
