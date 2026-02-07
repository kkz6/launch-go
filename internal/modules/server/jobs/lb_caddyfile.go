package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/formatter"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// Job type constants for load balancer Caddyfile operations
const (
	TypeInstallLBCaddyfile = "server:install_lb_caddyfile"
	TypeUpdateLBCaddyfile  = "server:update_lb_caddyfile"
	TypeRemoveLBCaddyfile  = "server:remove_lb_caddyfile"
)

// --- Install LB Caddyfile ---

// InstallLBCaddyfilePayload holds the data needed to install a new upstream Caddyfile
type InstallLBCaddyfilePayload struct {
	ServerID   string `json:"server_id"`
	UpstreamID string `json:"upstream_id"`
}

// InstallLBCaddyfileJob creates and installs an upstream Caddyfile on a load balancer server
type InstallLBCaddyfileJob struct {
	Deps    *JobDeps
	Payload InstallLBCaddyfilePayload
}

func NewInstallLBCaddyfileJob(p InstallLBCaddyfilePayload) pkgjobs.Handler {
	return &InstallLBCaddyfileJob{Deps: deps, Payload: p}
}

func (j *InstallLBCaddyfileJob) Handle(ctx context.Context) error {
	server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	upstream, err := j.Deps.Repos.LoadBalancerUpstream().FindByIDWithBackends(ctx, j.Payload.UpstreamID)
	if err != nil {
		return fmt.Errorf("failed to find upstream: %w", err)
	}

	// Generate Caddyfile content
	caddyfileContent := formatter.GenerateLBCaddyfile(upstream, upstream.Backends)
	caddyfilePath := formatter.UpstreamCaddyfilePath(upstream.ID)

	// Write the upstream Caddyfile
	task := tasks.UpdateUpstreamCaddyfile(tasks.UpdateUpstreamCaddyfileConfig{
		UpstreamID:       upstream.ID,
		CaddyfilePath:    caddyfilePath,
		CaddyfileContent: caddyfileContent,
	})

	result, err := j.Deps.RunTask(server, task).Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to install upstream Caddyfile: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to install upstream Caddyfile: %s", result.GetOutput())
	}

	// Update the Upstreams.caddy imports file
	if err := j.updateUpstreamsImports(ctx); err != nil {
		return fmt.Errorf("failed to update upstreams imports: %w", err)
	}

	j.Deps.Logger.Info().
		Str("upstream_id", upstream.ID).
		Str("server_id", j.Payload.ServerID).
		Str("address", upstream.Address).
		Msg("upstream Caddyfile installed successfully")

	j.Deps.BroadcastServerEvent(server, "upstream.caddyfile_installed", map[string]any{
		"upstream_id": upstream.ID,
		"server_id":   j.Payload.ServerID,
	})

	return nil
}

func (j *InstallLBCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("upstream_id", j.Payload.UpstreamID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to install upstream Caddyfile")
}

func (j *InstallLBCaddyfileJob) updateUpstreamsImports(ctx context.Context) error {
	server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return err
	}

	upstreams, err := j.Deps.Repos.LoadBalancerUpstream().FindByServerID(ctx, j.Payload.ServerID)
	if err != nil {
		return err
	}

	imports := make([]tasks.UpstreamImport, len(upstreams))
	for i, u := range upstreams {
		imports[i] = tasks.UpstreamImport{ID: u.ID}
	}

	task := tasks.UpdateUpstreamsImports(tasks.UpdateUpstreamsImportsConfig{
		Upstreams: imports,
	})

	result, err := j.Deps.RunTask(server, task).Dispatch(ctx)
	if err != nil {
		return err
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to update upstreams imports: %s", result.GetOutput())
	}

	return nil
}

// NewInstallLBCaddyfileTask creates an asynq task for installing an upstream Caddyfile
func NewInstallLBCaddyfileTask(serverID, upstreamID string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallLBCaddyfile, InstallLBCaddyfilePayload{
		ServerID:   serverID,
		UpstreamID: upstreamID,
	}, asynq.TaskID(pkgjobs.Dedup("install_lb_caddyfile", serverID, upstreamID)))
}

// --- Update LB Caddyfile ---

// UpdateLBCaddyfilePayload holds the data needed to update an existing upstream Caddyfile
type UpdateLBCaddyfilePayload struct {
	ServerID   string `json:"server_id"`
	UpstreamID string `json:"upstream_id"`
}

// UpdateLBCaddyfileJob regenerates and updates an upstream Caddyfile on a load balancer server
type UpdateLBCaddyfileJob struct {
	Deps    *JobDeps
	Payload UpdateLBCaddyfilePayload
}

func NewUpdateLBCaddyfileJob(p UpdateLBCaddyfilePayload) pkgjobs.Handler {
	return &UpdateLBCaddyfileJob{Deps: deps, Payload: p}
}

func (j *UpdateLBCaddyfileJob) Handle(ctx context.Context) error {
	server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	upstream, err := j.Deps.Repos.LoadBalancerUpstream().FindByIDWithBackends(ctx, j.Payload.UpstreamID)
	if err != nil {
		return fmt.Errorf("failed to find upstream: %w", err)
	}

	// Generate updated Caddyfile content
	caddyfileContent := formatter.GenerateLBCaddyfile(upstream, upstream.Backends)
	caddyfilePath := formatter.UpstreamCaddyfilePath(upstream.ID)

	// Write the updated upstream Caddyfile
	task := tasks.UpdateUpstreamCaddyfile(tasks.UpdateUpstreamCaddyfileConfig{
		UpstreamID:       upstream.ID,
		CaddyfilePath:    caddyfilePath,
		CaddyfileContent: caddyfileContent,
	})

	result, err := j.Deps.RunTask(server, task).Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to update upstream Caddyfile: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to update upstream Caddyfile: %s", result.GetOutput())
	}

	j.Deps.Logger.Info().
		Str("upstream_id", upstream.ID).
		Str("server_id", j.Payload.ServerID).
		Str("address", upstream.Address).
		Msg("upstream Caddyfile updated successfully")

	j.Deps.BroadcastServerEvent(server, "upstream.caddyfile_updated", map[string]any{
		"upstream_id": upstream.ID,
		"server_id":   j.Payload.ServerID,
	})

	return nil
}

func (j *UpdateLBCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("upstream_id", j.Payload.UpstreamID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to update upstream Caddyfile")
}

// NewUpdateLBCaddyfileTask creates an asynq task for updating an upstream Caddyfile
func NewUpdateLBCaddyfileTask(serverID, upstreamID string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUpdateLBCaddyfile, UpdateLBCaddyfilePayload{
		ServerID:   serverID,
		UpstreamID: upstreamID,
	}, asynq.TaskID(pkgjobs.Dedup("update_lb_caddyfile", serverID, upstreamID)))
}

// --- Remove LB Caddyfile ---

// RemoveLBCaddyfilePayload holds the data needed to remove an upstream Caddyfile
type RemoveLBCaddyfilePayload struct {
	ServerID   string `json:"server_id"`
	UpstreamID string `json:"upstream_id"`
}

// RemoveLBCaddyfileJob removes an upstream Caddyfile from a load balancer server
type RemoveLBCaddyfileJob struct {
	Deps    *JobDeps
	Payload RemoveLBCaddyfilePayload
}

func NewRemoveLBCaddyfileJob(p RemoveLBCaddyfilePayload) pkgjobs.Handler {
	return &RemoveLBCaddyfileJob{Deps: deps, Payload: p}
}

func (j *RemoveLBCaddyfileJob) Handle(ctx context.Context) error {
	server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	caddyfilePath := formatter.UpstreamCaddyfilePath(j.Payload.UpstreamID)

	// Remove the upstream Caddyfile
	task := tasks.RemoveUpstreamCaddyfile(tasks.RemoveUpstreamCaddyfileConfig{
		UpstreamID:    j.Payload.UpstreamID,
		CaddyfilePath: caddyfilePath,
	})

	result, err := j.Deps.RunTask(server, task).Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove upstream Caddyfile: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to remove upstream Caddyfile: %s", result.GetOutput())
	}

	// Update the Upstreams.caddy imports file
	if err := j.updateUpstreamsImports(ctx); err != nil {
		return fmt.Errorf("failed to update upstreams imports: %w", err)
	}

	j.Deps.Logger.Info().
		Str("upstream_id", j.Payload.UpstreamID).
		Str("server_id", j.Payload.ServerID).
		Msg("upstream Caddyfile removed successfully")

	j.Deps.BroadcastServerEvent(server, "upstream.caddyfile_removed", map[string]any{
		"upstream_id": j.Payload.UpstreamID,
		"server_id":   j.Payload.ServerID,
	})

	return nil
}

func (j *RemoveLBCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("upstream_id", j.Payload.UpstreamID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to remove upstream Caddyfile")
}

func (j *RemoveLBCaddyfileJob) updateUpstreamsImports(ctx context.Context) error {
	server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return err
	}

	upstreams, err := j.Deps.Repos.LoadBalancerUpstream().FindByServerID(ctx, j.Payload.ServerID)
	if err != nil {
		return err
	}

	// Filter out the removed upstream
	var imports []tasks.UpstreamImport
	for _, u := range upstreams {
		if u.ID != j.Payload.UpstreamID {
			imports = append(imports, tasks.UpstreamImport{ID: u.ID})
		}
	}

	task := tasks.UpdateUpstreamsImports(tasks.UpdateUpstreamsImportsConfig{
		Upstreams: imports,
	})

	result, err := j.Deps.RunTask(server, task).Dispatch(ctx)
	if err != nil {
		return err
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to update upstreams imports: %s", result.GetOutput())
	}

	return nil
}

// NewRemoveLBCaddyfileTask creates an asynq task for removing an upstream Caddyfile
func NewRemoveLBCaddyfileTask(serverID, upstreamID string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRemoveLBCaddyfile, RemoveLBCaddyfilePayload{
		ServerID:   serverID,
		UpstreamID: upstreamID,
	}, asynq.TaskID(pkgjobs.Dedup("remove_lb_caddyfile", serverID, upstreamID)))
}
