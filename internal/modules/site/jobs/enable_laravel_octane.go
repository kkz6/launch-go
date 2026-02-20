package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeEnableLaravelOctane = "site:enable_laravel_octane"

// EnableLaravelOctanePayload holds data for enabling Laravel Octane
type EnableLaravelOctanePayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// EnableLaravelOctaneJob enables Laravel Octane for a site
type EnableLaravelOctaneJob struct {
	Deps    *JobDeps
	Payload EnableLaravelOctanePayload
	FeatureJobHelpers

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewEnableLaravelOctaneJob creates a new EnableLaravelOctaneJob
func NewEnableLaravelOctaneJob(p EnableLaravelOctanePayload) pkgjobs.Handler {
	return &EnableLaravelOctaneJob{
		Deps:              deps,
		Payload:           p,
		FeatureJobHelpers: FeatureJobHelpers{Deps: deps},
	}
}

// Handle executes the enable Octane job
func (j *EnableLaravelOctaneJob) Handle(ctx context.Context) error {
	result, err := j.LoadAndValidate(ctx, j.Payload.SiteID, j.Payload.ServerID, FeatureOctane)
	if err != nil {
		return err
	}
	if result.AlreadyEnabled {
		return nil
	}

	site, server := result.Site, result.Server
	j.site = site
	j.server = server

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("server_id", server.ID).Msg("Enabling Laravel Octane")

	// Detect the Octane server type via SSH
	octaneServer, err := j.detectOctaneServer(ctx, site, server)
	if err != nil {
		return fmt.Errorf("failed to detect Octane server type: %w", err)
	}

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("octane_server", octaneServer).Msg("Detected Octane server type")

	// Install the server runtime
	if err := j.installServerRuntime(ctx, site, server, octaneServer); err != nil {
		return fmt.Errorf("failed to install Octane server runtime: %w", err)
	}

	// Allocate a port
	port, err := j.allocatePort(ctx, server.ID)
	if err != nil {
		return fmt.Errorf("failed to allocate Octane port: %w", err)
	}

	j.Deps.Logger.Info().Str("site_id", site.ID).Int("port", port).Msg("Allocated Octane port")

	// Create the supervisor/queue record for Octane
	userID := j.GetUserID(j.Payload.UserID, site)
	command := fmt.Sprintf("%s %s/artisan octane:start --server=%s --host=127.0.0.1 --port=%d",
		site.GetPhpBinary(), site.GetApplicationDirectory(), octaneServer, port)

	queue, err := j.CreateQueueRecord(ctx, site, server, command, userID)
	if err != nil {
		return err
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

	// Enable the feature with Octane metadata
	now := time.Now()
	feature := models.EnabledFeature{
		Name:         FeatureOctane,
		QueueID:      &queue.ID,
		OctanePort:   &port,
		OctaneServer: &octaneServer,
		EnabledAt:    &now,
	}

	site.AddEnabledFeature(feature)
	site.RemovePendingFeature(FeatureOctane)

	if err := j.Deps.Repos.Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
		"pending_features": site.PendingFeatures,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update site enabled_features")
	}

	j.BroadcastFeatureEnabled(server, FeatureOctane, site.ID, queue.ID)
	j.Deps.Logger.Info().Str("site_id", site.ID).Str("queue_id", queue.ID).Int("port", port).Msg("Laravel Octane enabled successfully")

	return nil
}

// detectOctaneServer runs an SSH task to detect which Octane server is configured
func (j *EnableLaravelOctaneJob) detectOctaneServer(ctx context.Context, site *models.Site, server *servermodels.Server) (string, error) {
	task := tasks.DetectOctaneServer(site.GetApplicationDirectory(), site.GetPhpBinary())
	result, err := j.Deps.RunTask(server, task).AsUser(site.User).Dispatch(ctx)
	if err != nil {
		return "frankenphp", nil
	}

	output := strings.TrimSpace(result.GetOutput())
	switch output {
	case "swoole", "openswoole":
		return "swoole", nil
	case "roadrunner":
		return "roadrunner", nil
	case "frankenphp":
		return "frankenphp", nil
	default:
		return "frankenphp", nil
	}
}

// installServerRuntime installs the appropriate Octane server binary/extension
func (j *EnableLaravelOctaneJob) installServerRuntime(ctx context.Context, site *models.Site, server *servermodels.Server, octaneServer string) error {
	switch octaneServer {
	case "frankenphp":
		task := tasks.InstallFrankenPhpBinary()
		result, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
		if err != nil {
			return err
		}
		if result.GetExitCode() != 0 {
			return fmt.Errorf("FrankenPHP installation failed with exit code %d", result.GetExitCode())
		}

	case "swoole":
		if site.PhpVersion == nil {
			return fmt.Errorf("PHP version is required to install Swoole extension")
		}
		userID := j.GetUserID(j.Payload.UserID, site)
		task, err := serverjobs.NewInstallPhpExtensionTask(server.ID, string(*site.PhpVersion), "swoole", &userID)
		if err != nil {
			return fmt.Errorf("failed to create install Swoole task: %w", err)
		}
		if err := j.Deps.DispatchTask(task); err != nil {
			return fmt.Errorf("failed to dispatch install Swoole job: %w", err)
		}

	case "roadrunner":
		task := tasks.InstallRoadRunnerBinary()
		result, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
		if err != nil {
			return err
		}
		if result.GetExitCode() != 0 {
			return fmt.Errorf("RoadRunner installation failed with exit code %d", result.GetExitCode())
		}
	}

	return nil
}

// allocatePort finds the first available port in the 8000-8999 range for Octane on a server
func (j *EnableLaravelOctaneJob) allocatePort(ctx context.Context, serverID string) (int, error) {
	sites, err := j.Deps.Repos.Site().FindByServer(ctx, serverID)
	if err != nil {
		return 0, fmt.Errorf("failed to query sites for port allocation: %w", err)
	}

	usedPorts := make(map[int]bool)
	for _, s := range sites {
		if port := s.GetOctanePort(); port != nil {
			usedPorts[*port] = true
		}
	}

	for port := 8000; port <= 8999; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range 8000-8999")
}

// dispatchUpdateCaddyfile dispatches the UpdateCaddyfile job to apply reverse_proxy config
func (j *EnableLaravelOctaneJob) dispatchUpdateCaddyfile(siteID string) error {
	task, err := NewUpdateCaddyfileTask(siteID, j.Payload.UserID)
	if err != nil {
		return err
	}
	return j.Deps.DispatchTask(task)
}

// Failed handles job failure
func (j *EnableLaravelOctaneJob) Failed(ctx context.Context, err error) {
	j.HandleFailure(ctx, err, j.Payload.SiteID, FeatureOctane, "Failed to enable Laravel Octane")
}

// NewEnableLaravelOctaneTask creates an enable Octane task
func NewEnableLaravelOctaneTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeEnableLaravelOctane, EnableLaravelOctanePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
