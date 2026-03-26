package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeEnableLaravelReverb = "site:enable_laravel_reverb"

// EnableLaravelReverbPayload holds data for enabling Laravel Reverb
type EnableLaravelReverbPayload struct {
	SiteID       string  `json:"site_id"`
	ServerID     string  `json:"server_id"`
	UserID       *string `json:"user_id,omitempty"`
	ConfigureEnv bool    `json:"configure_env,omitempty"`
}

// EnableLaravelReverbJob enables Laravel Reverb for a site
type EnableLaravelReverbJob struct {
	Deps    *JobDeps
	Payload EnableLaravelReverbPayload
	FeatureJobHelpers

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewEnableLaravelReverbJob creates a new EnableLaravelReverbJob
func NewEnableLaravelReverbJob(p EnableLaravelReverbPayload) pkgjobs.Handler {
	return &EnableLaravelReverbJob{
		Deps:              deps,
		Payload:           p,
		FeatureJobHelpers: FeatureJobHelpers{Deps: deps},
	}
}

// Handle executes the enable Reverb job
func (j *EnableLaravelReverbJob) Handle(ctx context.Context) error {
	result, err := j.LoadAndValidate(ctx, j.Payload.SiteID, j.Payload.ServerID, FeatureReverb)
	if err != nil {
		return err
	}
	if result.AlreadyEnabled {
		return nil
	}

	site, server := result.Site, result.Server
	j.site = site
	j.server = server

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("server_id", server.ID).Msg("Enabling Laravel Reverb")

	// Allocate a port in the 6001-6999 range
	port, err := j.allocatePort(ctx, server.ID)
	if err != nil {
		return fmt.Errorf("failed to allocate Reverb port: %w", err)
	}

	j.Deps.Logger.Info().Str("site_id", site.ID).Int("port", port).Msg("Allocated Reverb port")

	// Create the supervisor/queue record for Reverb
	userID := j.GetUserID(j.Payload.UserID, site)
	command := fmt.Sprintf("%s %s/artisan reverb:start --host=0.0.0.0 --port=%d",
		site.GetPhpBinary(), site.GetApplicationDirectory(), port)

	queue, err := j.CreateQueueRecord(ctx, site, server, command, userID)
	if err != nil {
		return err
	}

	// Configure .env variables if requested
	if j.Payload.ConfigureEnv {
		if err := j.configureEnv(ctx, site, server, port); err != nil {
			j.Deps.Logger.Error().Err(err).Msg("Failed to configure Reverb .env variables")
		}
	}

	// Install queue + update Caddyfile if site is already deployed
	if site.InstalledAt != nil {
		if err := j.DispatchInstallQueue(ctx, queue.ID, site.ID, j.Payload.UserID); err != nil {
			return err
		}

		if err := j.dispatchUpdateCaddyfile(site.ID); err != nil {
			j.Deps.Logger.Error().Err(err).Msg("Failed to dispatch update Caddyfile job")
		}
	}

	// Enable the feature with Reverb metadata
	now := time.Now()
	feature := models.EnabledFeature{
		Name:       FeatureReverb,
		QueueID:    &queue.ID,
		ReverbPort: &port,
		EnabledAt:  &now,
	}

	site.AddEnabledFeature(feature)
	site.RemovePendingFeature(FeatureReverb)

	if err := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update site enabled_features")
	}

	j.BroadcastFeatureEnabled(server, FeatureReverb, site.ID, queue.ID)
	j.Deps.Logger.Info().Str("site_id", site.ID).Str("queue_id", queue.ID).Int("port", port).Msg("Laravel Reverb enabled successfully")

	return nil
}

// allocatePort finds the first available port in the 6001-6999 range for Reverb on a server
func (j *EnableLaravelReverbJob) allocatePort(ctx context.Context, serverID string) (int, error) {
	sites, err := j.Deps.Repos.Site().FindByServer(ctx, serverID)
	if err != nil {
		return 0, fmt.Errorf("failed to query sites for port allocation: %w", err)
	}

	usedPorts := make(map[int]bool)
	for _, s := range sites {
		if port := s.GetReverbPort(); port != nil {
			usedPorts[*port] = true
		}
	}

	for port := 6001; port <= 6999; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range 6001-6999")
}

// configureEnv runs an SSH task to set Reverb .env variables
func (j *EnableLaravelReverbJob) configureEnv(ctx context.Context, site *models.Site, server *servermodels.Server, port int) error {
	task := tasks.ConfigureReverbEnv(site.GetApplicationDirectory(), site.Address, port)

	result, err := j.Deps.RunTask(server, task).AsUser(site.User).Dispatch(ctx)
	if err != nil {
		return err
	}

	if result.GetExitCode() != 0 {
		return fmt.Errorf("reverb env configuration failed with exit code %d", result.GetExitCode())
	}

	return nil
}

// dispatchUpdateCaddyfile dispatches the UpdateCaddyfile job to apply WebSocket proxy config
func (j *EnableLaravelReverbJob) dispatchUpdateCaddyfile(siteID string) error {
	task, err := NewUpdateCaddyfileTask(siteID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *EnableLaravelReverbJob) Failed(ctx context.Context, err error) {
	j.HandleFailure(ctx, err, j.Payload.SiteID, FeatureReverb, "Failed to enable Laravel Reverb")
}

// NewEnableLaravelReverbTask creates an enable Reverb task
func NewEnableLaravelReverbTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeEnableLaravelReverb, EnableLaravelReverbPayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}

// NewEnableLaravelReverbTaskWithOptions creates an enable Reverb task with additional options
func NewEnableLaravelReverbTaskWithOptions(siteID, serverID string, userID *string, configureEnv bool) (*asynq.Task, error) {
	return pkgjobs.Task(TypeEnableLaravelReverb, EnableLaravelReverbPayload{
		SiteID:       siteID,
		ServerID:     serverID,
		UserID:       userID,
		ConfigureEnv: configureEnv,
	})
}
