package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

var (
	ErrNotLoadBalancer     = fiberutil.BadRequest("Server is not a load balancer")
	ErrSiteAlreadyBalanced = fiberutil.Conflict("Site is already part of a load balancer upstream")
	ErrAddressMismatch     = fiberutil.BadRequest("Site address does not match upstream address")
	ErrUpstreamExists      = fiberutil.Conflict("An upstream with this address already exists on this server")
)

// LoadBalancerService provides business logic for load balancer operations
type LoadBalancerService struct {
	service.Base
	repos      contracts.RepositoryRegistry
	siteReader contracts.SiteReader
}

// NewLoadBalancerService creates a new LoadBalancerService
func NewLoadBalancerService(deps service.Dependencies, repos contracts.RepositoryRegistry, siteReader contracts.SiteReader) *LoadBalancerService {
	return &LoadBalancerService{
		Base:       service.NewBase(deps),
		repos:      repos,
		siteReader: siteReader,
	}
}

// ListUpstreams returns all upstreams for a load balancer server
func (s *LoadBalancerService) ListUpstreams(ctx context.Context, serverID, teamID string) ([]models.LoadBalancerUpstream, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	if !s.isLoadBalancer(server) {
		return nil, ErrNotLoadBalancer
	}

	return s.repos.LoadBalancerUpstream().FindByServerID(ctx, serverID)
}

// GetUpstream returns an upstream by ID with backends
func (s *LoadBalancerService) GetUpstream(ctx context.Context, serverID, teamID, upstreamID string) (*models.LoadBalancerUpstream, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	upstream, err := s.repos.LoadBalancerUpstream().FindByIDWithBackends(ctx, upstreamID)
	if err != nil {
		return nil, err
	}

	if upstream.ServerID != serverID {
		return nil, fiberutil.NotFound()
	}

	return upstream, nil
}

// CreateUpstream creates a new load balancer upstream
func (s *LoadBalancerService) CreateUpstream(ctx context.Context, serverID, teamID string, req *dto.CreateUpstreamRequest) (*models.LoadBalancerUpstream, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	if !s.isLoadBalancer(server) {
		return nil, ErrNotLoadBalancer
	}

	// Check for duplicate address on this server
	existing, err := s.repos.LoadBalancerUpstream().FindByServerIDAndAddress(ctx, serverID, req.Address)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUpstreamExists
	}

	// Apply defaults
	port := req.Port
	if port == 0 {
		port = 443
	}

	tlsSetting := req.TLSSetting
	if tlsSetting == "" {
		tlsSetting = "auto"
	}

	healthCheckPath := req.HealthCheckPath
	if healthCheckPath == "" {
		healthCheckPath = "/health"
	}

	healthCheckInterval := req.HealthCheckInterval
	if healthCheckInterval == "" {
		healthCheckInterval = "30s"
	}

	healthCheckTimeout := req.HealthCheckTimeout
	if healthCheckTimeout == "" {
		healthCheckTimeout = "10s"
	}

	upstream := &models.LoadBalancerUpstream{
		Name:                req.Name,
		Address:             req.Address,
		Port:                port,
		TLSSetting:          tlsSetting,
		LBPolicy:            types.LBPolicy(req.LBPolicy),
		HealthCheckPath:     healthCheckPath,
		HealthCheckInterval: healthCheckInterval,
		HealthCheckTimeout:  healthCheckTimeout,
	}
	upstream.ServerID = serverID
	upstream.TeamID = teamID

	if err := s.repos.LoadBalancerUpstream().Create(ctx, upstream); err != nil {
		return nil, fmt.Errorf("failed to create upstream: %w", err)
	}

	// If requested, auto-add existing sites as backends
	if req.AutoAddExistingSites && s.siteReader != nil {
		sites, err := s.siteReader.FindByAddressAndTeam(ctx, req.Address, teamID)
		if err == nil {
			for _, site := range sites {
				if site.LoadBalancedUpstreamID != nil {
					continue
				}
				_, _ = s.addBackendInternal(ctx, upstream, site.ID, site.ServerID, 8080)
			}
		}
	}

	// Dispatch job to install upstream Caddyfile on the LB server
	s.DispatchTask("InstallLBCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewInstallLBCaddyfileTask(serverID, upstream.ID)
	}, "server_id", serverID, "upstream_id", upstream.ID)

	// Broadcast upstream created event
	s.BroadcastToTeam(teamID, broadcast.UpstreamCreated, map[string]any{
		"upstream_id": upstream.ID,
		"server_id":   serverID,
		"address":     upstream.Address,
	})

	// Reload with backends
	return s.repos.LoadBalancerUpstream().FindByIDWithBackends(ctx, upstream.ID)
}

// UpdateUpstream updates an upstream's configuration
func (s *LoadBalancerService) UpdateUpstream(ctx context.Context, serverID, teamID, upstreamID string, req *dto.UpdateUpstreamRequest) (*models.LoadBalancerUpstream, error) {
	upstream, err := s.GetUpstream(ctx, serverID, teamID, upstreamID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]any)

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.TLSSetting != nil {
		updates["tls_setting"] = *req.TLSSetting
	}
	if req.LBPolicy != nil {
		updates["lb_policy"] = *req.LBPolicy
	}
	if req.HealthCheckPath != nil {
		updates["health_check_path"] = *req.HealthCheckPath
	}
	if req.HealthCheckInterval != nil {
		updates["health_check_interval"] = *req.HealthCheckInterval
	}
	if req.HealthCheckTimeout != nil {
		updates["health_check_timeout"] = *req.HealthCheckTimeout
	}

	if len(updates) == 0 {
		return upstream, nil
	}

	if err := s.repos.LoadBalancerUpstream().Update(ctx, upstreamID, updates); err != nil {
		return nil, err
	}

	// Dispatch job to update upstream Caddyfile (policy/health check changes affect config)
	s.DispatchTask("UpdateLBCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewUpdateLBCaddyfileTask(serverID, upstreamID)
	}, "server_id", serverID, "upstream_id", upstreamID)

	// Broadcast upstream updated event
	s.BroadcastToTeam(teamID, broadcast.UpstreamUpdated, map[string]any{
		"upstream_id": upstreamID,
		"server_id":   serverID,
	})

	return s.repos.LoadBalancerUpstream().FindByIDWithBackends(ctx, upstreamID)
}

// DeleteUpstream deletes an upstream and all its backends
func (s *LoadBalancerService) DeleteUpstream(ctx context.Context, serverID, teamID, upstreamID string) error {
	upstream, err := s.GetUpstream(ctx, serverID, teamID, upstreamID)
	if err != nil {
		return err
	}

	// Clear load_balanced_upstream_id on all associated sites and restore their Caddyfiles
	for _, backend := range upstream.Backends {
		if s.siteReader != nil {
			if err := s.siteReader.UpdateLoadBalancedUpstreamID(ctx, backend.SiteID, nil); err != nil {
				s.LogError(err, "failed to clear load balanced status on site", "site_id", backend.SiteID)
			}
		}

		// Dispatch site Caddyfile update (restores normal TLS mode)
		s.dispatchSiteCaddyfileUpdate(backend.SiteID)

		// Remove firewall rule on backend server
		s.dispatchRemoveLBFirewallRule(upstream, backend.ServerID, backend.Port)
	}

	// Delete all backends first
	if err := s.repos.LoadBalancerBackend().DeleteByUpstreamID(ctx, upstreamID); err != nil {
		return fmt.Errorf("failed to delete backends: %w", err)
	}

	// Delete the upstream
	if err := s.repos.LoadBalancerUpstream().Delete(ctx, upstreamID); err != nil {
		return fmt.Errorf("failed to delete upstream: %w", err)
	}

	// Dispatch job to remove upstream Caddyfile from the LB server
	s.DispatchTask("RemoveLBCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewRemoveLBCaddyfileTask(serverID, upstreamID)
	}, "server_id", serverID, "upstream_id", upstreamID)

	// Broadcast upstream deleted event
	s.BroadcastToTeam(teamID, broadcast.UpstreamDeleted, map[string]any{
		"upstream_id": upstreamID,
		"server_id":   serverID,
	})

	return nil
}

// AddBackend adds a site as a backend to an upstream
func (s *LoadBalancerService) AddBackend(ctx context.Context, serverID, teamID, upstreamID string, req *dto.AddBackendRequest) (*models.LoadBalancerBackend, error) {
	upstream, err := s.GetUpstream(ctx, serverID, teamID, upstreamID)
	if err != nil {
		return nil, err
	}

	// Validate the site exists and matches
	if s.siteReader == nil {
		return nil, fiberutil.Internal("Site reader not configured")
	}

	site, err := s.siteReader.FindByID(ctx, req.SiteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, fiberutil.NotFound()
	}

	// Validate site address matches upstream address
	if site.Address != upstream.Address {
		return nil, ErrAddressMismatch
	}

	// Validate site belongs to same team
	if site.TeamID != teamID {
		return nil, fiberutil.Forbidden("Site must belong to the same team")
	}

	// Validate site is not already load balanced
	if site.LoadBalancedUpstreamID != nil && *site.LoadBalancedUpstreamID != "" {
		return nil, ErrSiteAlreadyBalanced
	}

	port := req.Port
	if port == 0 {
		port = 8080
	}

	backend, err := s.addBackendInternal(ctx, upstream, req.SiteID, site.ServerID, port)
	if err != nil {
		return nil, err
	}

	// Dispatch job to update LB Caddyfile with new backend
	s.DispatchTask("UpdateLBCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewUpdateLBCaddyfileTask(serverID, upstreamID)
	}, "server_id", serverID, "upstream_id", upstreamID)

	// Dispatch backend site Caddyfile update (switches to port 8080 mode)
	s.dispatchSiteCaddyfileUpdate(req.SiteID)

	// Dispatch firewall rule to allow LB traffic on backend server
	s.dispatchAddLBFirewallRule(upstream, site.ServerID, port)

	// Broadcast backend added event
	s.BroadcastToTeam(teamID, broadcast.BackendAdded, map[string]any{
		"backend_id":  backend.ID,
		"upstream_id": upstreamID,
		"server_id":   serverID,
		"site_id":     req.SiteID,
	})

	return backend, nil
}

// addBackendInternal creates a backend record and marks the site as load balanced
func (s *LoadBalancerService) addBackendInternal(ctx context.Context, upstream *models.LoadBalancerUpstream, siteID, siteServerID string, port int) (*models.LoadBalancerBackend, error) {
	// Check for duplicate
	existing, err := s.repos.LoadBalancerBackend().FindByUpstreamAndSite(ctx, upstream.ID, siteID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	backend := &models.LoadBalancerBackend{
		UpstreamID:   upstream.ID,
		SiteID:       siteID,
		ServerID:     siteServerID,
		Port:         port,
		HealthStatus: types.HealthStatusUnknown,
	}

	if err := s.repos.LoadBalancerBackend().Create(ctx, backend); err != nil {
		return nil, fmt.Errorf("failed to create backend: %w", err)
	}

	// Mark site as load balanced
	if s.siteReader != nil {
		if err := s.siteReader.UpdateLoadBalancedUpstreamID(ctx, siteID, &upstream.ID); err != nil {
			s.LogError(err, "failed to mark site as load balanced", "site_id", siteID)
		}
	}

	return backend, nil
}

// UpdateBackend updates a backend's configuration
func (s *LoadBalancerService) UpdateBackend(ctx context.Context, serverID, teamID, upstreamID, backendID string, req *dto.UpdateBackendRequest) (*models.LoadBalancerBackend, error) {
	if _, err := s.GetUpstream(ctx, serverID, teamID, upstreamID); err != nil {
		return nil, err
	}

	backend, err := s.repos.LoadBalancerBackend().FindByID(ctx, backendID)
	if err != nil {
		return nil, err
	}

	if backend.UpstreamID != upstreamID {
		return nil, fiberutil.NotFound()
	}

	updates := make(map[string]any)

	if req.Port != nil {
		updates["port"] = *req.Port
	}
	if req.IsDown != nil {
		updates["is_down"] = *req.IsDown
	}

	if len(updates) == 0 {
		return backend, nil
	}

	if err := s.repos.LoadBalancerBackend().Update(ctx, backendID, updates); err != nil {
		return nil, err
	}

	// Dispatch job to update LB Caddyfile (port/down status affects config)
	s.DispatchTask("UpdateLBCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewUpdateLBCaddyfileTask(serverID, upstreamID)
	}, "server_id", serverID, "upstream_id", upstreamID)

	// Broadcast backend updated event
	s.BroadcastToTeam(teamID, broadcast.BackendUpdated, map[string]any{
		"backend_id":  backendID,
		"upstream_id": upstreamID,
		"server_id":   serverID,
	})

	return s.repos.LoadBalancerBackend().FindByID(ctx, backendID)
}

// RemoveBackend removes a site from an upstream
func (s *LoadBalancerService) RemoveBackend(ctx context.Context, serverID, teamID, upstreamID, backendID string) error {
	upstream, err := s.GetUpstream(ctx, serverID, teamID, upstreamID)
	if err != nil {
		return err
	}

	backend, err := s.repos.LoadBalancerBackend().FindByID(ctx, backendID)
	if err != nil {
		return err
	}

	if backend.UpstreamID != upstreamID {
		return fiberutil.NotFound()
	}

	// Clear load balanced status on site
	if s.siteReader != nil {
		if err := s.siteReader.UpdateLoadBalancedUpstreamID(ctx, backend.SiteID, nil); err != nil {
			s.LogError(err, "failed to clear load balanced status on site", "site_id", backend.SiteID)
		}
	}

	if err := s.repos.LoadBalancerBackend().Delete(ctx, backendID); err != nil {
		return err
	}

	// Dispatch job to update LB Caddyfile without this backend
	s.DispatchTask("UpdateLBCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewUpdateLBCaddyfileTask(serverID, upstreamID)
	}, "server_id", serverID, "upstream_id", upstreamID)

	// Dispatch backend site Caddyfile update (restores normal TLS mode)
	s.dispatchSiteCaddyfileUpdate(backend.SiteID)

	// Dispatch firewall rule removal on the backend server
	s.dispatchRemoveLBFirewallRule(upstream, backend.ServerID, backend.Port)

	// Broadcast backend removed event
	s.BroadcastToTeam(teamID, broadcast.BackendRemoved, map[string]any{
		"backend_id":  backendID,
		"upstream_id": upstreamID,
		"server_id":   serverID,
	})

	return nil
}

// ToggleBackendDown toggles a backend's down status
func (s *LoadBalancerService) ToggleBackendDown(ctx context.Context, serverID, teamID, upstreamID, backendID string) (*models.LoadBalancerBackend, error) {
	if _, err := s.GetUpstream(ctx, serverID, teamID, upstreamID); err != nil {
		return nil, err
	}

	backend, err := s.repos.LoadBalancerBackend().FindByID(ctx, backendID)
	if err != nil {
		return nil, err
	}

	if backend.UpstreamID != upstreamID {
		return nil, fiberutil.NotFound()
	}

	newIsDown := !backend.IsDown
	if err := s.repos.LoadBalancerBackend().Update(ctx, backendID, map[string]any{
		"is_down": newIsDown,
	}); err != nil {
		return nil, err
	}

	// Dispatch job to update LB Caddyfile (down status affects which backends are active)
	s.DispatchTask("UpdateLBCaddyfile", func() (*asynq.Task, error) {
		return jobs.NewUpdateLBCaddyfileTask(serverID, upstreamID)
	}, "server_id", serverID, "upstream_id", upstreamID)

	// Broadcast appropriate event
	event := broadcast.BackendMarkedDown
	if !newIsDown {
		event = broadcast.BackendMarkedUp
	}
	s.BroadcastToTeam(teamID, event, map[string]any{
		"backend_id":  backendID,
		"upstream_id": upstreamID,
		"server_id":   serverID,
		"is_down":     newIsDown,
	})

	return s.repos.LoadBalancerBackend().FindByID(ctx, backendID)
}

// CheckDomain checks if a domain is already used by sites in the team
func (s *LoadBalancerService) CheckDomain(ctx context.Context, serverID, teamID, address string) (*dto.CheckDomainResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	resp := &dto.CheckDomainResponse{
		Address: address,
	}

	if s.siteReader == nil {
		return resp, nil
	}

	sites, err := s.siteReader.FindByAddressAndTeam(ctx, address, teamID)
	if err != nil {
		return resp, nil
	}

	if len(sites) > 0 {
		resp.Exists = true
		resp.Sites = make([]dto.DomainCheckSite, len(sites))
		for i, site := range sites {
			resp.Sites[i] = dto.DomainCheckSite{
				ID:       site.ID,
				ServerID: site.ServerID,
				Address:  site.Address,
				Type:     site.Type,
			}
		}
		resp.Warning = "Creating this upstream will route traffic through the load balancer. Existing site Caddyfiles will be reconfigured."
	}

	return resp, nil
}

// ListBackends returns all backends for an upstream
func (s *LoadBalancerService) ListBackends(ctx context.Context, serverID, teamID, upstreamID string) ([]models.LoadBalancerBackend, error) {
	if _, err := s.GetUpstream(ctx, serverID, teamID, upstreamID); err != nil {
		return nil, err
	}

	return s.repos.LoadBalancerBackend().FindByUpstreamID(ctx, upstreamID)
}

// GetUpstreamHealth returns health status of all backends in an upstream
func (s *LoadBalancerService) GetUpstreamHealth(ctx context.Context, serverID, teamID, upstreamID string) (*dto.UpstreamHealthResponse, error) {
	upstream, err := s.GetUpstream(ctx, serverID, teamID, upstreamID)
	if err != nil {
		return nil, err
	}

	backends := make([]dto.BackendHealthStatus, len(upstream.Backends))
	healthy := 0
	for i, b := range upstream.Backends {
		backends[i] = dto.BackendHealthStatus{
			BackendID:         b.ID,
			ServerID:          b.ServerID,
			SiteID:            b.SiteID,
			Port:              b.Port,
			IsDown:            b.IsDown,
			HealthStatus:      string(b.HealthStatus),
			LastHealthCheckAt: b.LastHealthCheckAt,
		}
		if b.HealthStatus == types.HealthStatusHealthy && !b.IsDown {
			healthy++
		}
	}

	return &dto.UpstreamHealthResponse{
		UpstreamID:      upstream.ID,
		Address:         upstream.Address,
		TotalBackends:   len(upstream.Backends),
		HealthyBackends: healthy,
		Backends:        backends,
	}, nil
}

// TriggerHealthCheck dispatches an on-demand health check for a specific upstream's backends
func (s *LoadBalancerService) TriggerHealthCheck(ctx context.Context, serverID, teamID, upstreamID string) error {
	if _, err := s.GetUpstream(ctx, serverID, teamID, upstreamID); err != nil {
		return err
	}

	s.DispatchTask("CheckLBBackendHealth", func() (*asynq.Task, error) {
		return jobs.NewCheckLBBackendHealthTaskForUpstream(upstreamID)
	}, "upstream_id", upstreamID)

	return nil
}

// isLoadBalancer checks if a server is a load balancer type
func (s *LoadBalancerService) isLoadBalancer(server *models.Server) bool {
	return server.Type != nil && types.ServerType(*server.Type) == types.ServerTypeLoadBalancer
}

// dispatchSiteCaddyfileUpdate dispatches a site:update_caddyfile job to regenerate
// the backend site's Caddyfile (switches to/from load-balanced port 8080 mode).
func (s *LoadBalancerService) dispatchSiteCaddyfileUpdate(siteID string) {
	type sitePayload struct {
		SiteID string `json:"site_id"`
	}

	s.DispatchTask("UpdateBackendSiteCaddyfile", func() (*asynq.Task, error) {
		data, err := json.Marshal(sitePayload{SiteID: siteID})
		if err != nil {
			return nil, err
		}
		return asynq.NewTask("site:update_caddyfile", data,
			asynq.TaskID(pkgjobs.Dedup("update_caddyfile", siteID)),
		), nil
	}, "site_id", siteID)
}

// dispatchAddLBFirewallRule dispatches a firewall rule job to allow LB traffic on the backend server.
func (s *LoadBalancerService) dispatchAddLBFirewallRule(upstream *models.LoadBalancerUpstream, backendServerID string, port int) {
	lbIP := s.getUpstreamServerIP(upstream)
	if lbIP == "" {
		s.LogWarn("skipping firewall rule: LB server IP unavailable", "upstream_id", upstream.ID, "backend_server_id", backendServerID)
		return
	}

	s.DispatchTask("AddLBFirewallRule", func() (*asynq.Task, error) {
		return jobs.NewAddLBFirewallRuleTask(backendServerID, lbIP, port)
	}, "backend_server_id", backendServerID, "lb_ip", lbIP, "port", port)
}

// dispatchRemoveLBFirewallRule dispatches a firewall rule removal job on the backend server.
func (s *LoadBalancerService) dispatchRemoveLBFirewallRule(upstream *models.LoadBalancerUpstream, backendServerID string, port int) {
	lbIP := s.getUpstreamServerIP(upstream)
	if lbIP == "" {
		s.LogWarn("skipping firewall rule removal: LB server IP unavailable", "upstream_id", upstream.ID, "backend_server_id", backendServerID)
		return
	}

	s.DispatchTask("RemoveLBFirewallRule", func() (*asynq.Task, error) {
		return jobs.NewRemoveLBFirewallRuleTask(backendServerID, lbIP, port)
	}, "backend_server_id", backendServerID, "lb_ip", lbIP, "port", port)
}

// getUpstreamServerIP returns the load balancer server's public IPv4 address.
func (s *LoadBalancerService) getUpstreamServerIP(upstream *models.LoadBalancerUpstream) string {
	if upstream.Server != nil && upstream.Server.PublicIPv4 != nil {
		return *upstream.Server.PublicIPv4
	}

	return ""
}
