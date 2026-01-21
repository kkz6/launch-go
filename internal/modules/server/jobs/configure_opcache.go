package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
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
	pkgjobs.BaseJob[*JobContext, ConfigureOpcachePayload]
}

// Handle processes the job
func (j *ConfigureOpcacheJob) Handle(ctx context.Context) error {
	// Find the service
	service, err := j.Ctx.Repos().Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}

	// Find the server
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Verify it's a PHP service
	if service.Type != enums.ServiceTypePhp {
		return fmt.Errorf("service is not a PHP installation")
	}

	// Get the PHP version from the software
	software := enums.Software(service.Software)
	version := software.GetVersion()

	// Mark as configuring
	typeData := map[string]any{
		"opcache": map[string]any{
			"status": "configuring",
		},
	}
	if err := j.Ctx.Repos().Service().UpdateWithTypeData(ctx, service.ID, service.Status, typeData); err != nil {
		j.Ctx.LogError(err, "Failed to update service status")
	}

	// Create and run the configure opcache task
	task := tasks.ConfigureOpcache(version, j.Payload.Settings)
	result, err := j.Ctx.ForServer(server).RunTask(task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		// Mark as failed
		typeData["opcache"].(map[string]any)["status"] = "failed"
		typeData["opcache"].(map[string]any)["error"] = err.Error()
		_ = j.Ctx.Repos().Service().UpdateWithTypeData(ctx, service.ID, service.Status, typeData)
		return fmt.Errorf("failed to configure OPcache: %w", err)
	}

	if !result.IsSuccessful() {
		// Mark as failed
		typeData["opcache"].(map[string]any)["status"] = "failed"
		typeData["opcache"].(map[string]any)["error"] = result.GetOutput()
		_ = j.Ctx.Repos().Service().UpdateWithTypeData(ctx, service.ID, service.Status, typeData)
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
	if err := j.Ctx.Repos().Service().UpdateWithTypeData(ctx, service.ID, service.Status, typeData); err != nil {
		j.Ctx.LogError(err, "Failed to save OPcache settings")
	}

	j.Ctx.LogInfo("OPcache configuration completed",
		"service_id", service.ID,
		"server_id", server.ID,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(server, "opcache.configured", map[string]any{
		"service_id": service.ID,
		"server_id":  server.ID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *ConfigureOpcacheJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to configure OPcache",
		"service_id", j.Payload.ServiceID,
		"server_id", j.Payload.ServerID,
	)
}

// NewConfigureOpcacheJob creates a new ConfigureOpcacheJob with the given context and payload.
func NewConfigureOpcacheJob(ctx *JobContext, payload ConfigureOpcachePayload) *ConfigureOpcacheJob {
	return &ConfigureOpcacheJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewConfigureOpcacheTask creates an asynq task for configuring OPcache
func NewConfigureOpcacheTask(serverID, serviceID string, settings map[string]string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeConfigureOpcache, ConfigureOpcachePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Settings:  settings,
	})
}
