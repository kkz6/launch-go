package services

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

var (
	ErrNotLoadBalancer     = fiberutil.BadRequest("Server is not a load balancer")
	ErrSiteAlreadyBalanced = fiberutil.Conflict("Site is already part of a load balancer upstream")
	ErrAddressMismatch     = fiberutil.BadRequest("Site address does not match upstream address")
	ErrUpstreamExists      = fiberutil.Conflict("An upstream with this address already exists on this server")
)

// LoadBalancerService provides business logic for load balancer operations
type LoadBalancerService struct {
	repos      contracts.RepositoryRegistry
	siteReader contracts.SiteReader
}

// NewLoadBalancerService creates a new LoadBalancerService
func NewLoadBalancerService(repos contracts.RepositoryRegistry, siteReader contracts.SiteReader) *LoadBalancerService {
	return &LoadBalancerService{
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

	return s.repos.LoadBalancerUpstream().FindByIDWithBackends(ctx, upstreamID)
}

// DeleteUpstream deletes an upstream and all its backends
func (s *LoadBalancerService) DeleteUpstream(ctx context.Context, serverID, teamID, upstreamID string) error {
	upstream, err := s.GetUpstream(ctx, serverID, teamID, upstreamID)
	if err != nil {
		return err
	}

	// Clear load_balanced_upstream_id on all associated sites
	if s.siteReader != nil {
		for _, backend := range upstream.Backends {
			_ = s.siteReader.UpdateLoadBalancedUpstreamID(ctx, backend.SiteID, nil)
		}
	}

	// Delete all backends first
	if err := s.repos.LoadBalancerBackend().DeleteByUpstreamID(ctx, upstreamID); err != nil {
		return fmt.Errorf("failed to delete backends: %w", err)
	}

	// Delete the upstream
	if err := s.repos.LoadBalancerUpstream().Delete(ctx, upstreamID); err != nil {
		return fmt.Errorf("failed to delete upstream: %w", err)
	}

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

	return s.addBackendInternal(ctx, upstream, req.SiteID, site.ServerID, port)
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
		_ = s.siteReader.UpdateLoadBalancedUpstreamID(ctx, siteID, &upstream.ID)
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

	return s.repos.LoadBalancerBackend().FindByID(ctx, backendID)
}

// RemoveBackend removes a site from an upstream
func (s *LoadBalancerService) RemoveBackend(ctx context.Context, serverID, teamID, upstreamID, backendID string) error {
	if _, err := s.GetUpstream(ctx, serverID, teamID, upstreamID); err != nil {
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
		_ = s.siteReader.UpdateLoadBalancedUpstreamID(ctx, backend.SiteID, nil)
	}

	return s.repos.LoadBalancerBackend().Delete(ctx, backendID)
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

	if err := s.repos.LoadBalancerBackend().Update(ctx, backendID, map[string]any{
		"is_down": !backend.IsDown,
	}); err != nil {
		return nil, err
	}

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

// isLoadBalancer checks if a server is a load balancer type
func (s *LoadBalancerService) isLoadBalancer(server *models.Server) bool {
	return server.Type != nil && types.ServerType(*server.Type) == types.ServerTypeLoadBalancer
}
