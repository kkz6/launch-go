package site

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for sites
type Handler struct {
	service *Service
}

// NewHandler creates a new site handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List returns all sites for a server
func (h *Handler) List(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	sites, err := h.service.List(c.Context(), serverID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch sites")
	}

	result := make([]SiteResponse, len(sites))
	for i, site := range sites {
		result[i] = ToSiteResponse(&site)
	}

	return response.OK(c, "Sites retrieved", result)
}

// Create creates a new site
func (h *Handler) Create(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	userID := c.Locals("userID").(string)
	username := c.Locals("username").(string)

	var req CreateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	site, err := h.service.Create(c.Context(), serverID, userID, username, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Site created", ToSiteResponse(site))
}

// Show returns a single site
func (h *Handler) Show(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	site, err := h.service.FindByID(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.InternalError(c, "Failed to fetch site")
	}

	return response.OK(c, "Site retrieved", ToSiteResponse(site))
}

// Update updates a site
func (h *Handler) Update(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req UpdateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	site, err := h.service.Update(c.Context(), siteID, serverID, userID, &req)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Site updated", ToSiteResponse(site))
}

// Delete deletes a site
func (h *Handler) Delete(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.service.Delete(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Site deletion initiated", nil)
}

// Deploy triggers a new deployment
func (h *Handler) Deploy(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	deployment, err := h.service.Deploy(c.Context(), siteID, serverID, userID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		if errors.Is(err, ErrPendingDeployment) {
			return response.Error(c, fiber.StatusConflict, "A deployment is already in progress")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Deployment started", ToDeploymentResponse(deployment))
}

// Rollback rolls back to a previous deployment
func (h *Handler) Rollback(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	targetDeploymentID := c.Params("deploymentId")
	userID := c.Locals("userID").(string)

	deployment, err := h.service.Rollback(c.Context(), siteID, serverID, targetDeploymentID, userID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		if errors.Is(err, ErrDeploymentNotFound) {
			return response.NotFound(c, "Deployment not found")
		}
		if errors.Is(err, ErrPendingDeployment) {
			return response.Error(c, fiber.StatusConflict, "A deployment is already in progress")
		}
		if errors.Is(err, ErrRollbackNotSupported) {
			return response.Error(c, fiber.StatusBadRequest, "Rollback is only available for sites with zero downtime deployment enabled")
		}
		if errors.Is(err, ErrInvalidRollbackTarget) {
			return response.Error(c, fiber.StatusBadRequest, "Can only rollback to a finished deployment")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Rollback initiated", ToDeploymentResponse(deployment))
}

// ListDeployments returns all deployments for a site
func (h *Handler) ListDeployments(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	deployments, err := h.service.ListDeployments(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.InternalError(c, "Failed to fetch deployments")
	}

	result := make([]DeploymentResponse, len(deployments))
	for i, deployment := range deployments {
		result[i] = ToDeploymentResponse(&deployment)
	}

	return response.OK(c, "Deployments retrieved", result)
}

// ShowDeployment returns a single deployment
func (h *Handler) ShowDeployment(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	deploymentID := c.Params("deploymentId")

	deployment, err := h.service.FindDeployment(c.Context(), deploymentID, siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		if errors.Is(err, ErrDeploymentNotFound) {
			return response.NotFound(c, "Deployment not found")
		}
		return response.InternalError(c, "Failed to fetch deployment")
	}

	return response.OK(c, "Deployment retrieved", ToDeploymentResponse(deployment))
}

// CancelQueuedDeployments cancels all queued deployments
func (h *Handler) CancelQueuedDeployments(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	count, err := h.service.CancelQueuedDeployments(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.InternalError(c, "Failed to cancel deployments")
	}

	return response.OK(c, "Queued deployments cancelled", map[string]int64{"cancelled": count})
}

// EnableAutoDeployment enables auto-deployment
func (h *Handler) EnableAutoDeployment(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.service.EnableAutoDeployment(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		if errors.Is(err, ErrSourceControlNotConnected) {
			return response.Error(c, fiber.StatusBadRequest, "Source control is not connected")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Auto-deployment enabled", nil)
}

// DisableAutoDeployment disables auto-deployment
func (h *Handler) DisableAutoDeployment(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.service.DisableAutoDeployment(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Auto-deployment disabled", nil)
}

// EnableAutoRestartQueue enables auto-restart for queue workers
func (h *Handler) EnableAutoRestartQueue(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.service.EnableAutoRestartQueue(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Auto-restart queue enabled", nil)
}

// DisableAutoRestartQueue disables auto-restart for queue workers
func (h *Handler) DisableAutoRestartQueue(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.service.DisableAutoRestartQueue(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Auto-restart queue disabled", nil)
}

// RegenerateDeployToken regenerates the deploy token
func (h *Handler) RegenerateDeployToken(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.service.RegenerateDeployToken(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Deploy token regenerated", nil)
}

// GetDeletionSummary returns a summary of resources to be deleted
func (h *Handler) GetDeletionSummary(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	summary, err := h.service.GetDeletionSummary(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.InternalError(c, "Failed to get deletion summary")
	}

	return response.OK(c, "Deletion summary retrieved", summary)
}

// UpdateSSL updates SSL settings
func (h *Handler) UpdateSSL(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req UpdateSSLRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.UpdateSSL(c.Context(), siteID, serverID, userID, &req); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "SSL settings updated", nil)
}

// ListCertificates returns all certificates for a site
func (h *Handler) ListCertificates(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	certs, err := h.service.ListCertificates(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.InternalError(c, "Failed to fetch certificates")
	}

	result := make([]CertificateResponse, len(certs))
	for i, cert := range certs {
		result[i] = ToCertificateResponse(&cert)
	}

	return response.OK(c, "Certificates retrieved", result)
}

// CreateQueue creates a new queue worker
func (h *Handler) CreateQueue(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req CreateQueueRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	queue, err := h.service.CreateQueue(c.Context(), siteID, serverID, userID, &req)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Queue created", ToQueueResponse(queue))
}

// ListQueues returns all queues for a site
func (h *Handler) ListQueues(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	queues, err := h.service.ListQueues(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.InternalError(c, "Failed to fetch queues")
	}

	result := make([]QueueResponse, len(queues))
	for i, queue := range queues {
		result[i] = ToQueueResponse(&queue)
	}

	return response.OK(c, "Queues retrieved", result)
}

// DeleteQueue deletes a queue
func (h *Handler) DeleteQueue(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	queueID := c.Params("queueId")

	if err := h.service.DeleteQueue(c.Context(), queueID, siteID, serverID); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		if errors.Is(err, ErrQueueNotFound) {
			return response.NotFound(c, "Queue not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Queue deletion initiated", nil)
}

// CreateCommand creates and executes a command
func (h *Handler) CreateCommand(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req CreateCommandRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	cmd, err := h.service.CreateCommand(c.Context(), siteID, serverID, userID, &req)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		if errors.Is(err, ErrSiteNotInstalled) {
			return response.Error(c, fiber.StatusBadRequest, "Site is not installed")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Command created", ToCommandResponse(cmd))
}

// ListCommands returns all commands for a site
func (h *Handler) ListCommands(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	commands, err := h.service.ListCommands(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.InternalError(c, "Failed to fetch commands")
	}

	result := make([]CommandResponse, len(commands))
	for i, cmd := range commands {
		result[i] = ToCommandResponse(&cmd)
	}

	return response.OK(c, "Commands retrieved", result)
}

// CreateRedirect creates a new redirect
func (h *Handler) CreateRedirect(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req CreateRedirectRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	redirect, err := h.service.CreateRedirect(c.Context(), siteID, serverID, userID, &req)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Redirect created", ToRedirectResponse(redirect))
}

// ListRedirects returns all redirects for a site
func (h *Handler) ListRedirects(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	redirects, err := h.service.ListRedirects(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.InternalError(c, "Failed to fetch redirects")
	}

	result := make([]RedirectResponse, len(redirects))
	for i, redirect := range redirects {
		result[i] = ToRedirectResponse(&redirect)
	}

	return response.OK(c, "Redirects retrieved", result)
}

// DeleteRedirect deletes a redirect
func (h *Handler) DeleteRedirect(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	redirectID := c.Params("redirectId")

	if err := h.service.DeleteRedirect(c.Context(), redirectID, siteID, serverID); err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		if errors.Is(err, ErrRedirectNotFound) {
			return response.NotFound(c, "Redirect not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Redirect deleted", nil)
}

// UpdateDeploymentSettings updates deployment settings
func (h *Handler) UpdateDeploymentSettings(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req UpdateDeploymentSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	// Convert to UpdateSiteRequest
	updateReq := &UpdateSiteRequest{
		DeployNotificationEmail:      req.DeployNotificationEmail,
		SharedDirectories:            req.SharedDirectories,
		SharedFiles:                  req.SharedFiles,
		WriteableDirectories:         req.WriteableDirectories,
		HookBeforeUpdatingRepository: req.HookBeforeUpdatingRepository,
		HookAfterUpdatingRepository:  req.HookAfterUpdatingRepository,
		HookBeforeMakingCurrent:      req.HookBeforeMakingCurrent,
		HookAfterMakingCurrent:       req.HookAfterMakingCurrent,
		DeploymentReleasesRetention:  req.DeploymentReleasesRetention,
		QueueDeployments:             req.QueueDeployments,
	}

	site, err := h.service.Update(c.Context(), siteID, serverID, userID, updateReq)
	if err != nil {
		if errors.Is(err, ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Deployment settings updated", ToSiteResponse(site))
}
