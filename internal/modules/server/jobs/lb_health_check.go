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

// CheckLBBackendHealthPayload is empty — the job checks all upstreams
type CheckLBBackendHealthPayload struct{}

// CheckLBBackendHealthJob polls health endpoints for all load balancer backends
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
	// Find all LB servers that have upstreams
	upstreams, err := j.findAllUpstreams(ctx)
	if err != nil {
		return fmt.Errorf("failed to find upstreams: %w", err)
	}

	if len(upstreams) == 0 {
		return nil
	}

	var wg sync.WaitGroup

	for i := range upstreams {
		upstream := &upstreams[i]

		for k := range upstream.Backends {
			backend := &upstream.Backends[k]

			if backend.IsDown {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				j.checkBackendHealth(ctx, upstream, backend)
			}()
		}
	}

	wg.Wait()

	return nil
}

func (j *CheckLBBackendHealthJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("LB backend health check job failed")
}

// findAllUpstreams returns all upstreams with their backends and servers preloaded
func (j *CheckLBBackendHealthJob) findAllUpstreams(ctx context.Context) ([]models.LoadBalancerUpstream, error) {
	var upstreams []models.LoadBalancerUpstream
	err := j.Deps.DB.WithContext(ctx).
		Preload("Backends").
		Preload("Backends.Server").
		Find(&upstreams).Error

	return upstreams, err
}

// checkBackendHealth makes an HTTP request to the backend's health endpoint
func (j *CheckLBBackendHealthJob) checkBackendHealth(ctx context.Context, upstream *models.LoadBalancerUpstream, backend *models.LoadBalancerBackend) {
	if backend.Server == nil || backend.Server.PublicIPv4 == nil {
		return
	}

	healthURL := fmt.Sprintf("http://%s:%d%s", *backend.Server.PublicIPv4, backend.Port, upstream.HealthCheckPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		j.updateHealthStatus(ctx, upstream, backend, types.HealthStatusUnhealthy)
		return
	}

	// Set Host header so Caddy routes correctly
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
