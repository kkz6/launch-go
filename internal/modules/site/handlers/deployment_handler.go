package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DeploymentHandler handles HTTP requests for deployments
type DeploymentHandler struct {
	deploymentService *services.DeploymentService
}

// NewDeploymentHandler creates a new deployment handler
func NewDeploymentHandler(deploymentService *services.DeploymentService) *DeploymentHandler {
	return &DeploymentHandler{deploymentService: deploymentService}
}

// Deploy triggers a new deployment
func (h *DeploymentHandler) Deploy(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	deployment, err := h.deploymentService.Deploy(c.Context(), siteID, serverID, userID)
	if err != nil {
		return fiberctx.Abort(err)
	}

	return fiberctx.OK(c, "Deployment started", dto.ToDeploymentResponse(deployment))
}

// Rollback rolls back to a previous deployment
func (h *DeploymentHandler) Rollback(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	targetDeploymentID, err := fiberctx.GetDeploymentID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	deployment, err := h.deploymentService.Rollback(c.Context(), siteID, serverID, targetDeploymentID, userID)
	if err != nil {
		return fiberctx.Abort(err)
	}

	return fiberctx.OK(c, "Rollback initiated", dto.ToDeploymentResponse(deployment))
}

// ListDeployments returns all deployments for a site
func (h *DeploymentHandler) ListDeployments(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	deployments, err := h.deploymentService.List(c.Context(), siteID, serverID)
	if err != nil {
		return fiberctx.Abort(err)
	}

	result := make([]dto.DeploymentResponse, len(deployments))
	for i, deployment := range deployments {
		result[i] = dto.ToDeploymentResponse(&deployment)
	}

	return fiberctx.OK(c, "Deployments retrieved", result)
}

// ShowDeployment returns a single deployment
func (h *DeploymentHandler) ShowDeployment(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	deploymentID, err := fiberctx.GetDeploymentID(c)
	if err != nil {
		return err
	}

	deployment, err := h.deploymentService.FindByID(c.Context(), deploymentID, siteID, serverID)
	if err != nil {
		return fiberctx.Abort(err)
	}

	return fiberctx.OK(c, "Deployment retrieved", dto.ToDeploymentResponse(deployment))
}

// CancelQueuedDeployments cancels all queued deployments
func (h *DeploymentHandler) CancelQueuedDeployments(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	count, err := h.deploymentService.CancelQueued(c.Context(), siteID, serverID)
	if err != nil {
		return fiberctx.Abort(err)
	}

	return fiberctx.OK(c, "Queued deployments cancelled", map[string]int64{"cancelled": count})
}

// EnableAutoDeployment enables auto-deployment
func (h *DeploymentHandler) EnableAutoDeployment(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	if err := h.deploymentService.EnableAutoDeployment(c.Context(), siteID, serverID); err != nil {
		return fiberctx.Abort(err)
	}

	return fiberctx.OK(c, "Auto-deployment enabled", nil)
}

// DisableAutoDeployment disables auto-deployment
func (h *DeploymentHandler) DisableAutoDeployment(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	if err := h.deploymentService.DisableAutoDeployment(c.Context(), siteID, serverID); err != nil {
		return fiberctx.Abort(err)
	}

	return fiberctx.OK(c, "Auto-deployment disabled", nil)
}
