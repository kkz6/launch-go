package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
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
	Deps    *JobDeps
	Payload SetDefaultPhpPayload

	server  *models.Server
	service *models.InstalledService
}

func NewSetDefaultPhpJob(p SetDefaultPhpPayload) pkgjobs.Handler {
	return &SetDefaultPhpJob{Deps: deps, Payload: p}
}

func (j *SetDefaultPhpJob) Handle(ctx context.Context) error {
	var err error

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.service, err = j.Deps.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", j.Payload.Version).
		Msg("setting default PHP version")

	// Run the update alternatives task
	task := tasks.UpdateAlternatives(j.Payload.Version)

	result, err := j.Deps.RunTask(j.server, task).
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
	if err := j.Deps.Repos.Service().UnsetDefaultPhp(ctx, j.Payload.ServerID); err != nil {
		j.Deps.Logger.Error().Err(err).
			Msg("failed to unset existing default PHP")
	}

	// Set the new default
	if err := j.Deps.Repos.Service().SetDefault(ctx, j.Payload.ServiceID, true); err != nil {
		return fmt.Errorf("failed to update service default status: %w", err)
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("service_id", j.service.ID).
		Str("version", j.Payload.Version).
		Msg("default PHP version set successfully")

	j.Deps.BroadcastServerEvent(j.server, "php.default_changed", map[string]any{
		"server_id":  j.server.ID,
		"service_id": j.service.ID,
		"version":    j.Payload.Version,
	})

	return nil
}

func (j *SetDefaultPhpJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("version", j.Payload.Version).
		Msg("failed to set default PHP version")
}

func NewSetDefaultPhpTask(serverID, serviceID, version string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeSetDefaultPhp, SetDefaultPhpPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Version:   version,
		UserID:    userID,
	})
}
