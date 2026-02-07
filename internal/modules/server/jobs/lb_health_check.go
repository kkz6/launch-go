package jobs

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeCheckLBBackendHealth = "server:check_lb_backend_health"

// maxConcurrentHealthChecks limits parallel HTTP health check requests
const maxConcurrentHealthChecks = 20

// CheckLBBackendHealthPayload optionally scopes to a single upstream
type CheckLBBackendHealthPayload struct {
	UpstreamID string `json:"upstream_id,omitempty"`
}

// CheckLBBackendHealthJob polls health endpoints for load balancer backends.
// Health checks are routed through the LB server's reverse proxy (not directly
// to backends) since backend port 8080 is firewalled to the LB IP only.
type CheckLBBackendHealthJob struct {
	Deps    *JobDeps
	Payload CheckLBBackendHealthPayload
	client  *http.Client
}

func NewCheckLBBackendHealthJob(p CheckLBBackendHealthPayload) pkgjobs.Handler {
	return &CheckLBBackendHealthJob{
		Deps:    deps,
		Payload: p,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (j *CheckLBBackendHealthJob) Handle(ctx context.Context) error {
	upstreams, err := j.findUpstreams(ctx)
	if err != nil {
		return fmt.Errorf("failed to find upstreams: %w", err)
	}

	if len(upstreams) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentHealthChecks)

	for i := range upstreams {
		upstream := &upstreams[i]

		// Need the LB server IP to route health checks through the LB
		lbIP := ""
		if upstream.Server != nil && upstream.Server.PublicIPv4 != nil {
			lbIP = *upstream.Server.PublicIPv4
		}
		if lbIP == "" {
			j.Deps.Logger.Warn().
				Str("upstream_id", upstream.ID).
				Msg("skipping health check: LB server has no public IP")
			continue
		}

		for k := range upstream.Backends {
			backend := &upstream.Backends[k]

			if backend.IsDown {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				j.checkBackendHealth(ctx, upstream, backend, lbIP)
			}()
		}
	}

	wg.Wait()

	return nil
}

func (j *CheckLBBackendHealthJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("LB backend health check job failed")
}

// findUpstreams returns upstreams with backends and servers preloaded.
// If UpstreamID is set, only that upstream is returned.
func (j *CheckLBBackendHealthJob) findUpstreams(ctx context.Context) ([]models.LoadBalancerUpstream, error) {
	var upstreams []models.LoadBalancerUpstream

	query := j.Deps.DB.WithContext(ctx).
		Preload("Server").
		Preload("Backends").
		Preload("Backends.Server")

	if j.Payload.UpstreamID != "" {
		query = query.Where("id = ?", j.Payload.UpstreamID)
	}

	err := query.Find(&upstreams).Error
	return upstreams, err
}

// checkBackendHealth checks a backend's health by routing through the LB server's
// reverse proxy. This avoids firewall/Caddy IP restrictions on backend port 8080.
func (j *CheckLBBackendHealthJob) checkBackendHealth(ctx context.Context, upstream *models.LoadBalancerUpstream, backend *models.LoadBalancerBackend, lbIP string) {
	// Route through the LB server: http://<lb_ip>:<upstream_port><health_path>
	// with Host header set to the upstream address so Caddy routes correctly.
	healthURL := fmt.Sprintf("http://%s:%d%s", lbIP, upstream.Port, upstream.HealthCheckPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		j.updateHealthStatus(ctx, upstream, backend, types.HealthStatusUnhealthy)
		return
	}

	// Set Host header so the LB's Caddy routes to the correct upstream
	req.Host = upstream.Address

	resp, err := j.client.Do(req)
	if err != nil {
		j.updateHealthStatus(ctx, upstream, backend, types.HealthStatusUnhealthy)
		return
	}
	defer resp.Body.Close()

	newStatus := types.HealthStatusHealthy
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		newStatus = types.HealthStatusUnhealthy
	}

	j.updateHealthStatus(ctx, upstream, backend, newStatus)
}

// updateHealthStatus updates the backend's health status and broadcasts changes
func (j *CheckLBBackendHealthJob) updateHealthStatus(ctx context.Context, upstream *models.LoadBalancerUpstream, backend *models.LoadBalancerBackend, newStatus types.HealthStatus) {
	oldStatus := backend.HealthStatus
	now := time.Now()

	if err := j.Deps.Repos.LoadBalancerBackend().Update(ctx, backend.ID, map[string]any{
		"health_status":        newStatus,
		"last_health_check_at": now,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Str("backend_id", backend.ID).Msg("failed to update backend health status")
		return
	}

	if oldStatus != newStatus {
		j.Deps.Logger.Info().
			Str("backend_id", backend.ID).
			Str("upstream_id", upstream.ID).
			Str("old_status", string(oldStatus)).
			Str("new_status", string(newStatus)).
			Msg("backend health status changed")

		j.Deps.BroadcastServerEvent(backend.Server, broadcast.BackendHealthChanged, map[string]any{
			"backend_id":    backend.ID,
			"upstream_id":   upstream.ID,
			"server_id":     upstream.ServerID,
			"health_status": newStatus,
			"previous":      oldStatus,
		})
	}
}

// NewCheckLBBackendHealthTask creates an asynq task for the health check job
func NewCheckLBBackendHealthTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeCheckLBBackendHealth, CheckLBBackendHealthPayload{})
}

// NewCheckLBBackendHealthTaskForUpstream creates a scoped health check task for a single upstream
func NewCheckLBBackendHealthTaskForUpstream(upstreamID string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeCheckLBBackendHealth, CheckLBBackendHealthPayload{
		UpstreamID: upstreamID,
	}, asynq.TaskID(pkgjobs.Dedup("check_lb_health", upstreamID)))
}
