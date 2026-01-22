package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeConfigureOpcache = "server:configure_opcache"

type ConfigureOpcachePayload struct {
	ServerID  string            `json:"server_id"`
	ServiceID string            `json:"service_id"`
	Settings  map[string]string `json:"settings"`
}

// ConfigureOpcacheJob configures OPcache settings for a PHP version.
type ConfigureOpcacheJob struct {
	Deps    *JobDeps
	Payload ConfigureOpcachePayload

	server  *models.Server
	service *models.InstalledService
}

func NewConfigureOpcacheJob(p ConfigureOpcachePayload) pkgjobs.Handler {
	return &ConfigureOpcacheJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *ConfigureOpcacheJob) Handle(ctx context.Context) error {
	var err error

	// Find the service
	j.service, err = j.Deps.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Verify it's a PHP service
	if j.service.Type != types.ServiceTypePhp {
		return fmt.Errorf("service is not a PHP installation")
	}

	// Get the PHP version from the software
	software := types.Software(j.service.Software)
	version := software.GetVersion()

	// Mark as configuring
	typeData := map[string]any{
		"opcache": map[string]any{
			"status": "configuring",
		},
	}
	if err := j.Deps.Repos.Service().UpdateWithTypeData(ctx, j.service.ID, j.service.Status, typeData); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("failed to update service status")
	}

	// Create and run the configure opcache task
	task := tasks.ConfigureOpcache(version, j.Payload.Settings)
	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		// Mark as failed
		typeData["opcache"].(map[string]any)["status"] = "failed"
		typeData["opcache"].(map[string]any)["error"] = err.Error()
		_ = j.Deps.Repos.Service().UpdateWithTypeData(ctx, j.service.ID, j.service.Status, typeData)
		return fmt.Errorf("failed to configure OPcache: %w", err)
	}

	if !result.IsSuccessful() {
		// Mark as failed
		typeData["opcache"].(map[string]any)["status"] = "failed"
		typeData["opcache"].(map[string]any)["error"] = result.GetOutput()
		_ = j.Deps.Repos.Service().UpdateWithTypeData(ctx, j.service.ID, j.service.Status, typeData)
		return fmt.Errorf("failed to configure OPcache: %s", result.GetOutput())
	}

	// Save the OPcache settings to TypeData
	opcacheSettings := map[string]any{
		"status": "configured",
	}
	// Convert settings map to proper types for storage
	for k, v := range j.Payload.Settings {
		switch k {
		case "enable":
			opcacheSettings["enabled"] = v == "1"
		case "enable_cli":
			opcacheSettings["enable_cli"] = v == "1"
		case "validate_timestamps":
			opcacheSettings["validate_timestamps"] = v == "1"
		case "save_comments":
			opcacheSettings["save_comments"] = v == "1"
		case "memory_consumption", "interned_strings_buffer", "max_accelerated_files", "revalidate_freq":
			opcacheSettings[k] = v
		case "jit_buffer_size":
			opcacheSettings["jit_buffer_size"] = v
			opcacheSettings["jit_enabled"] = true
		case "jit":
			opcacheSettings["jit_mode"] = v
		}
	}

	typeData["opcache"] = opcacheSettings
	if err := j.Deps.Repos.Service().UpdateWithTypeData(ctx, j.service.ID, j.service.Status, typeData); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("failed to save OPcache settings")
	}

	j.Deps.Logger.Info().
		Str("service_id", j.service.ID).
		Str("server_id", j.server.ID).
		Msg("OPcache configuration completed")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.server, "opcache.configured", map[string]any{
		"service_id": j.service.ID,
		"server_id":  j.server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ConfigureOpcacheJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("service_id", j.Payload.ServiceID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to configure OPcache")
}

// NewConfigureOpcacheTask creates an asynq task for configuring OPcache
func NewConfigureOpcacheTask(serverID, serviceID string, settings map[string]string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeConfigureOpcache, ConfigureOpcachePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Settings:  settings,
	}, asynq.TaskID(pkgjobs.Dedup("configure_opcache", serverID, serviceID)))
}
