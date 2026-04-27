package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// LoadBalancerHandler holds the load-balancer endpoints that do not fit
// a generic route helper. Standard CRUD on upstreams and backends is
// wired directly to the framework helpers (single + double-nested) in
// routes.go.
type LoadBalancerHandler struct {
	lbService *services.LoadBalancerService
}

// NewLoadBalancerHandler creates a new load balancer handler.
func NewLoadBalancerHandler(lbService *services.LoadBalancerService) *LoadBalancerHandler {
	return &LoadBalancerHandler{lbService: lbService}
}

// CheckDomain checks if a domain is already used by existing sites.
// Carries a query-string `address` parameter so it does not fit Index.
func (h *LoadBalancerHandler) CheckDomain(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	address := c.Query("address")
	if address == "" {
		return fiberctx.BadRequest("Address query parameter is required")
	}
	result, err := h.lbService.CheckDomain(c.Context(), serverID, teamID, address)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Domain check completed", result)
}

// ToggleBackendDown toggles a backend's down status. Action that
// returns the updated DTO — does not fit ActionItemDoubleNested (no
// payload) or UpdateDoubleNested (no body).
func (h *LoadBalancerHandler) ToggleBackendDown(c *fiber.Ctx) error {
	teamID, serverID, upstreamID, err := fiberctx.GetTeamServerAndEntityID(c, "upstreamId")
	if err != nil {
		return err
	}
	backendID := c.Params("backendId")
	if backendID == "" {
		return fiberctx.BadRequest("Backend ID is required")
	}
	backend, err := h.lbService.ToggleBackendDown(c.Context(), serverID, teamID, upstreamID, backendID)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Backend status toggled", dto.ToLoadBalancerBackendResponse(backend))
}
