package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
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
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	deployment, err := h.deploymentService.Deploy(c.Context(), siteID, serverID, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		if errors.Is(err, services.ErrPendingDeployment) {
			return response.Error(c, fiber.StatusConflict, "A deployment is already in progress")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Deployment started", dto.ToDeploymentResponse(deployment))
}

// Rollback rolls back to a previous deployment
func (h *DeploymentHandler) Rollback(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	targetDeploymentID := c.Params("deploymentId")
	userID := c.Locals("userID").(string)

	deployment, err := h.deploymentService.Rollback(c.Context(), siteID, serverID, targetDeploymentID, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		if errors.Is(err, repositories.ErrDeploymentNotFound) {
			return response.NotFound(c, "Deployment not found")
		}

		if errors.Is(err, services.ErrPendingDeployment) {
			return response.Error(c, fiber.StatusConflict, "A deployment is already in progress")
		}

		if errors.Is(err, services.ErrRollbackNotSupported) {
			return response.Error(c, fiber.StatusBadRequest, "Rollback is only available for sites with zero downtime deployment enabled")
		}

		if errors.Is(err, services.ErrInvalidRollbackTarget) {
			return response.Error(c, fiber.StatusBadRequest, "Can only rollback to a finished deployment")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Rollback initiated", dto.ToDeploymentResponse(deployment))
}

// ListDeployments returns all deployments for a site
func (h *DeploymentHandler) ListDeployments(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	deployments, err := h.deploymentService.List(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.InternalError(c, "Failed to fetch deployments")
	}

	result := make([]dto.DeploymentResponse, len(deployments))
	for i, deployment := range deployments {
		result[i] = dto.ToDeploymentResponse(&deployment)
	}

	return response.OK(c, "Deployments retrieved", result)
}

// ShowDeployment returns a single deployment
func (h *DeploymentHandler) ShowDeployment(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	deploymentID := c.Params("deploymentId")

	deployment, err := h.deploymentService.FindByID(c.Context(), deploymentID, siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		if errors.Is(err, repositories.ErrDeploymentNotFound) {
			return response.NotFound(c, "Deployment not found")
		}

		return response.InternalError(c, "Failed to fetch deployment")
	}

	return response.OK(c, "Deployment retrieved", dto.ToDeploymentResponse(deployment))
}

// CancelQueuedDeployments cancels all queued deployments
func (h *DeploymentHandler) CancelQueuedDeployments(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	count, err := h.deploymentService.CancelQueued(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.InternalError(c, "Failed to cancel deployments")
	}

	return response.OK(c, "Queued deployments cancelled", map[string]int64{"cancelled": count})
}

// EnableAutoDeployment enables auto-deployment
func (h *DeploymentHandler) EnableAutoDeployment(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.deploymentService.EnableAutoDeployment(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		if errors.Is(err, services.ErrSourceControlNotConnected) {
			return response.Error(c, fiber.StatusBadRequest, "Source control is not connected")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Auto-deployment enabled", nil)
}

// DisableAutoDeployment disables auto-deployment
func (h *DeploymentHandler) DisableAutoDeployment(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.deploymentService.DisableAutoDeployment(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Auto-deployment disabled", nil)
}
