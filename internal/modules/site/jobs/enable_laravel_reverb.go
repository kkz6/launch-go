package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeEnableLaravelReverb = "site:enable_laravel_reverb"

// EnableLaravelReverbPayload holds data for enabling Laravel Reverb
type EnableLaravelReverbPayload struct {
	SiteID          string  `json:"site_id"`
	ServerID        string  `json:"server_id"`
	UserID          *string `json:"user_id,omitempty"`
	ConfigureEnv    bool    `json:"configure_env,omitempty"`
	UpdateCaddyfile bool    `json:"update_caddyfile,omitempty"`
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

	// Create a daemon record for the Reverb process
	command := fmt.Sprintf("%s %s/artisan reverb:start --host=0.0.0.0 --port=%d",
		site.GetPhpBinary(), site.GetApplicationDirectory(), port)

	daemon, err := j.createDaemonRecord(ctx, site, server, command)
	if err != nil {
		return err
	}

	// Install daemon + optional env/Caddyfile config if site is already deployed
	if site.InstalledAt != nil {
		if j.Payload.ConfigureEnv {
			if err := j.configureEnv(ctx, site, server, port); err != nil {
				j.Deps.Logger.Error().Err(err).Msg("Failed to configure Reverb .env variables")
			}
		}

		if err := j.dispatchInstallDaemon(daemon.ID, server.ID); err != nil {
			// Cleanup daemon record if dispatch fails
			_ = j.Deps.ServerRepos.Daemon().Delete(ctx, daemon.ID)
			return fmt.Errorf("failed to dispatch install daemon job: %w", err)
		}

		if j.Payload.UpdateCaddyfile {
			if err := j.dispatchUpdateCaddyfile(site.ID); err != nil {
				j.Deps.Logger.Error().Err(err).Msg("Failed to dispatch update Caddyfile job")
			}
		}
	}

	// Enable the feature with Reverb metadata
	now := time.Now()
	feature := models.EnabledFeature{
		Name:       FeatureReverb,
		DaemonID:   &daemon.ID,
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

	j.Deps.BroadcastServerEvent(server, "site.reverb_enabled", map[string]interface{}{
		"site_id":   site.ID,
		"daemon_id": daemon.ID,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("daemon_id", daemon.ID).Int("port", port).Msg("Laravel Reverb enabled successfully")

	return nil
}

// createDaemonRecord creates a daemon record for the Reverb process
func (j *EnableLaravelReverbJob) createDaemonRecord(ctx context.Context, site *models.Site, server *servermodels.Server, command string) (*servermodels.Daemon, error) {
	daemon := &servermodels.Daemon{
		Command:         command,
		User:            site.User,
		Processes:       1,
		StopWaitSeconds: 10,
		StopSignal:      "SIGTERM",
	}
	daemon.ServerID = server.ID

	if err := j.Deps.ServerRepos.Daemon().Create(ctx, daemon); err != nil {
		return nil, fmt.Errorf("failed to create daemon: %w", err)
	}

	return daemon, nil
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

// dispatchInstallDaemon dispatches the InstallDaemon job
func (j *EnableLaravelReverbJob) dispatchInstallDaemon(daemonID, serverID string) error {
	task, err := serverjobs.NewInstallDaemonTask(serverID, daemonID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
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

// NewEnableLaravelReverbTask creates an enable Reverb task with default options (env + caddyfile)
func NewEnableLaravelReverbTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeEnableLaravelReverb, EnableLaravelReverbPayload{
		SiteID:          siteID,
		ServerID:        serverID,
		UserID:          userID,
		ConfigureEnv:    true,
		UpdateCaddyfile: true,
	})
}

// NewEnableLaravelReverbTaskWithOptions creates an enable Reverb task with explicit options
func NewEnableLaravelReverbTaskWithOptions(siteID, serverID string, userID *string, configureEnv, updateCaddyfile bool) (*asynq.Task, error) {
	return pkgjobs.Task(TypeEnableLaravelReverb, EnableLaravelReverbPayload{
		SiteID:          siteID,
		ServerID:        serverID,
		UserID:          userID,
		ConfigureEnv:    configureEnv,
		UpdateCaddyfile: updateCaddyfile,
	})
}
