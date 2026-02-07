package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// LoadBalancerHandler handles load balancer-related HTTP requests
type LoadBalancerHandler struct {
	lbService *services.LoadBalancerService
}

// NewLoadBalancerHandler creates a new load balancer handler
func NewLoadBalancerHandler(lbService *services.LoadBalancerService) *LoadBalancerHandler {
	return &LoadBalancerHandler{lbService: lbService}
}

// ListUpstreams returns all upstreams for a load balancer server
func (h *LoadBalancerHandler) ListUpstreams(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreams, err := h.lbService.ListUpstreams(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch upstreams")
	}

	return fiberctx.OK(c, "Upstreams retrieved", pkgdto.TransformSlice(upstreams, dto.ToLoadBalancerUpstreamResponse))
}

// CreateUpstream creates a new upstream
func (h *LoadBalancerHandler) CreateUpstream(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateUpstreamRequest](c)
	if err != nil {
		return err
	}

	upstream, err := h.lbService.CreateUpstream(c.Context(), serverID, teamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Upstream created", dto.ToLoadBalancerUpstreamResponse(upstream))
}

// ShowUpstream returns a single upstream with backends
func (h *LoadBalancerHandler) ShowUpstream(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	upstream, err := h.lbService.GetUpstream(c.Context(), serverID, teamID, upstreamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch upstream")
	}

	return fiberctx.OK(c, "Upstream retrieved", dto.ToLoadBalancerUpstreamResponse(upstream))
}

// UpdateUpstream updates an upstream's configuration
func (h *LoadBalancerHandler) UpdateUpstream(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateUpstreamRequest](c)
	if err != nil {
		return err
	}

	upstream, err := h.lbService.UpdateUpstream(c.Context(), serverID, teamID, upstreamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Upstream updated", dto.ToLoadBalancerUpstreamResponse(upstream))
}

// DeleteUpstream deletes an upstream and all its backends
func (h *LoadBalancerHandler) DeleteUpstream(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	if err := h.lbService.DeleteUpstream(c.Context(), serverID, teamID, upstreamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.NoContent(c)
}

// CheckDomain checks if a domain is already used by existing sites
func (h *LoadBalancerHandler) CheckDomain(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	address := c.Query("address")
	if address == "" {
		return fiberctx.BadRequest("Address query parameter is required")
	}

	result, err := h.lbService.CheckDomain(c.Context(), serverID, teamID, address)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to check domain")
	}

	return fiberctx.OK(c, "Domain check completed", result)
}

// ListBackends returns all backends for an upstream
func (h *LoadBalancerHandler) ListBackends(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	backends, err := h.lbService.ListBackends(c.Context(), serverID, teamID, upstreamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch backends")
	}

	return fiberctx.OK(c, "Backends retrieved", pkgdto.TransformSlice(backends, dto.ToLoadBalancerBackendResponse))
}

// AddBackend adds a site as a backend to an upstream
func (h *LoadBalancerHandler) AddBackend(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	req, err := fiberctx.MustParseAndValidate[dto.AddBackendRequest](c)
	if err != nil {
		return err
	}

	backend, err := h.lbService.AddBackend(c.Context(), serverID, teamID, upstreamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Backend added", dto.ToLoadBalancerBackendResponse(backend))
}

// UpdateBackend updates a backend's configuration
func (h *LoadBalancerHandler) UpdateBackend(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	backendID := c.Params("backendId")
	if backendID == "" {
		return fiberctx.BadRequest("Backend ID is required")
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateBackendRequest](c)
	if err != nil {
		return err
	}

	backend, err := h.lbService.UpdateBackend(c.Context(), serverID, teamID, upstreamID, backendID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Backend updated", dto.ToLoadBalancerBackendResponse(backend))
}

// RemoveBackend removes a site from an upstream
func (h *LoadBalancerHandler) RemoveBackend(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	backendID := c.Params("backendId")
	if backendID == "" {
		return fiberctx.BadRequest("Backend ID is required")
	}

	if err := h.lbService.RemoveBackend(c.Context(), serverID, teamID, upstreamID, backendID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.NoContent(c)
}

// GetUpstreamHealth returns the health status of all backends
func (h *LoadBalancerHandler) GetUpstreamHealth(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	health, err := h.lbService.GetUpstreamHealth(c.Context(), serverID, teamID, upstreamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch upstream health")
	}

	return fiberctx.OK(c, "Upstream health retrieved", health)
}

// TriggerHealthCheck triggers an on-demand health check for an upstream's backends
func (h *LoadBalancerHandler) TriggerHealthCheck(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	if err := h.lbService.TriggerHealthCheck(c.Context(), serverID, teamID, upstreamID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Health check triggered", nil)
}

// ToggleBackendDown toggles a backend's down status
func (h *LoadBalancerHandler) ToggleBackendDown(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	upstreamID := c.Params("upstreamId")
	if upstreamID == "" {
		return fiberctx.BadRequest("Upstream ID is required")
	}

	backendID := c.Params("backendId")
	if backendID == "" {
		return fiberctx.BadRequest("Backend ID is required")
	}

	backend, err := h.lbService.ToggleBackendDown(c.Context(), serverID, teamID, upstreamID, backendID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Backend status toggled", dto.ToLoadBalancerBackendResponse(backend))
}
