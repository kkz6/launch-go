package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
)

// Helper-shaped methods for routes/load_balancer. Each method wraps the
// existing model-returning service method below it, adapts the signature
// to the framework helper convention (id-first, then parents, then
// teamID/userID, then req), and returns response DTOs at the HTTP
// boundary. The underlying business logic stays in the original
// methods; this file is the adapter layer.

// Upstreams lists upstreams for a server. Matches IndexNestedFunc:
// (ctx, parentID=serverID, teamID).
func (s *LoadBalancerService) Upstreams(ctx context.Context, serverID, teamID string) ([]dto.LoadBalancerUpstreamResponse, error) {
	upstreams, err := s.ListUpstreams(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.LoadBalancerUpstreamResponse, len(upstreams))
	for i := range upstreams {
		out[i] = dto.ToLoadBalancerUpstreamResponse(&upstreams[i])
	}
	return out, nil
}

// UpstreamShow returns a single upstream. Matches ShowNestedFunc:
// (ctx, id=upstreamID, parentID=serverID, teamID).
func (s *LoadBalancerService) UpstreamShow(ctx context.Context, upstreamID, serverID, teamID string) (dto.LoadBalancerUpstreamResponse, error) {
	upstream, err := s.GetUpstream(ctx, serverID, teamID, upstreamID)
	if err != nil {
		return dto.LoadBalancerUpstreamResponse{}, err
	}
	return dto.ToLoadBalancerUpstreamResponse(upstream), nil
}

// UpstreamCreate creates a new upstream. Matches CreateNestedFunc.
func (s *LoadBalancerService) UpstreamCreate(ctx context.Context, serverID, teamID, userID string, req *dto.CreateUpstreamRequest) (dto.LoadBalancerUpstreamResponse, error) {
	_ = userID
	upstream, err := s.CreateUpstream(ctx, serverID, teamID, req)
	if err != nil {
		return dto.LoadBalancerUpstreamResponse{}, err
	}
	return dto.ToLoadBalancerUpstreamResponse(upstream), nil
}

// UpstreamUpdate updates an upstream. Matches UpdateNestedFunc.
func (s *LoadBalancerService) UpstreamUpdate(ctx context.Context, upstreamID, serverID, teamID, userID string, req *dto.UpdateUpstreamRequest) (dto.LoadBalancerUpstreamResponse, error) {
	_ = userID
	upstream, err := s.UpdateUpstream(ctx, serverID, teamID, upstreamID, req)
	if err != nil {
		return dto.LoadBalancerUpstreamResponse{}, err
	}
	return dto.ToLoadBalancerUpstreamResponse(upstream), nil
}

// UpstreamDelete deletes an upstream. Matches DeleteNestedFunc.
func (s *LoadBalancerService) UpstreamDelete(ctx context.Context, upstreamID, serverID, teamID, userID string) error {
	_ = userID
	return s.DeleteUpstream(ctx, serverID, teamID, upstreamID)
}

// UpstreamHealth returns health for an upstream. Matches ShowNestedFunc.
func (s *LoadBalancerService) UpstreamHealth(ctx context.Context, upstreamID, serverID, teamID string) (*dto.UpstreamHealthResponse, error) {
	return s.GetUpstreamHealth(ctx, serverID, teamID, upstreamID)
}

// UpstreamHealthCheck triggers a health check. Matches ActionItemNestedFunc.
func (s *LoadBalancerService) UpstreamHealthCheck(ctx context.Context, upstreamID, serverID, teamID, userID string) error {
	_ = userID
	return s.TriggerHealthCheck(ctx, serverID, teamID, upstreamID)
}

// Backends lists backends for an upstream. Matches IndexDoubleNestedFunc:
// (ctx, parentID=upstreamID, grandparentID=serverID, teamID).
func (s *LoadBalancerService) Backends(ctx context.Context, upstreamID, serverID, teamID string) ([]dto.LoadBalancerBackendResponse, error) {
	backends, err := s.ListBackends(ctx, serverID, teamID, upstreamID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.LoadBalancerBackendResponse, len(backends))
	for i := range backends {
		out[i] = dto.ToLoadBalancerBackendResponse(&backends[i])
	}
	return out, nil
}

// BackendCreate adds a backend. Matches CreateDoubleNestedFunc.
func (s *LoadBalancerService) BackendCreate(ctx context.Context, upstreamID, serverID, teamID, userID string, req *dto.AddBackendRequest) (dto.LoadBalancerBackendResponse, error) {
	_ = userID
	backend, err := s.AddBackend(ctx, serverID, teamID, upstreamID, req)
	if err != nil {
		return dto.LoadBalancerBackendResponse{}, err
	}
	return dto.ToLoadBalancerBackendResponse(backend), nil
}

// BackendUpdate updates a backend. Matches UpdateDoubleNestedFunc.
func (s *LoadBalancerService) BackendUpdate(ctx context.Context, backendID, upstreamID, serverID, teamID, userID string, req *dto.UpdateBackendRequest) (dto.LoadBalancerBackendResponse, error) {
	_ = userID
	backend, err := s.UpdateBackend(ctx, serverID, teamID, upstreamID, backendID, req)
	if err != nil {
		return dto.LoadBalancerBackendResponse{}, err
	}
	return dto.ToLoadBalancerBackendResponse(backend), nil
}

// BackendDelete removes a backend. Matches DeleteDoubleNestedFunc.
func (s *LoadBalancerService) BackendDelete(ctx context.Context, backendID, upstreamID, serverID, teamID, userID string) error {
	_ = userID
	return s.RemoveBackend(ctx, serverID, teamID, upstreamID, backendID)
}
