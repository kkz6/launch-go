package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DeploymentHandler holds the deployment endpoints that do not fit a
// generic route helper. List and Show are wired directly to the
// double-nested helpers in routes.go; the bespoke handlers below are
// for actions that return DTOs (Deploy, Rollback), shaped responses
// (CancelQueued returns {cancelled: count}), or branching on the body
// (ToggleAutoDeployment).
type DeploymentHandler struct {
	deploymentService *services.DeploymentService
}

// NewDeploymentHandler creates a new deployment handler.
func NewDeploymentHandler(deploymentService *services.DeploymentService) *DeploymentHandler {
	return &DeploymentHandler{deploymentService: deploymentService}
}

// Deploy triggers a new deployment and returns the new deployment DTO.
// Action with returned payload — does not fit ActionDoubleNested.
func (h *DeploymentHandler) Deploy(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	deployment, err := h.deploymentService.Deploy(c.Context(), siteID, serverID, userID)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Deployment started", dto.ToDeploymentResponse(deployment))
}

// Rollback rolls back to a previous deployment and returns the new
// deployment DTO. Action with returned payload + extra :deploymentId
// path parameter — bespoke.
func (h *DeploymentHandler) Rollback(c *fiber.Ctx) error {
	serverID, siteID, targetDeploymentID, err := fiberctx.GetServerSiteAndEntityID(c, "deploymentId")
	if err != nil {
		return err
	}
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	deployment, err := h.deploymentService.Rollback(c.Context(), siteID, serverID, targetDeploymentID, userID)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Rollback initiated", dto.ToDeploymentResponse(deployment))
}

// CancelQueuedDeployments cancels all queued deployments and returns
// the count of cancelled rows. Custom-shape response.
func (h *DeploymentHandler) CancelQueuedDeployments(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}
	count, err := h.deploymentService.CancelQueued(c.Context(), siteID, serverID)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Queued deployments cancelled", map[string]int64{"cancelled": count})
}

// ToggleAutoDeployment toggles auto-deployment based on the request
// body — branches on the parsed `enabled` flag and dispatches to the
// matching service method.
func (h *DeploymentHandler) ToggleAutoDeployment(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiberctx.RespondBadRequest(c, "Invalid request body")
	}
	if body.Enabled {
		if err := h.deploymentService.EnableAutoDeployment(c.Context(), siteID, serverID); err != nil {
			return err
		}
		return fiberctx.OK(c, "Auto-deployment enabled", nil)
	}
	if err := h.deploymentService.DisableAutoDeployment(c.Context(), siteID, serverID); err != nil {
		return err
	}
	return fiberctx.OK(c, "Auto-deployment disabled", nil)
}
