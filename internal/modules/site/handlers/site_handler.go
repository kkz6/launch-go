package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// SiteHandler handles HTTP requests for sites
type SiteHandler struct {
	siteService *services.SiteService
}

// NewSiteHandler creates a new site handler
func NewSiteHandler(siteService *services.SiteService) *SiteHandler {
	return &SiteHandler{siteService: siteService}
}

// List returns all sites for a server
func (h *SiteHandler) List(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	sites, err := h.siteService.List(c.Context(), serverID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch sites")
	}

	result := make([]dto.SiteResponse, len(sites))
	for i, site := range sites {
		result[i] = dto.ToSiteResponse(&site)
	}

	return response.OK(c, "Sites retrieved", result)
}

// Create creates a new site
func (h *SiteHandler) Create(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	userID := c.Locals("userID").(string)
	username := c.Locals("username").(string)

	var req dto.CreateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	site, err := h.siteService.Create(c.Context(), serverID, userID, username, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Site created", dto.ToSiteResponse(site))
}

// Show returns a single site
func (h *SiteHandler) Show(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	site, err := h.siteService.FindByID(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.InternalError(c, "Failed to fetch site")
	}

	return response.OK(c, "Site retrieved", dto.ToSiteResponse(site))
}

// Update updates a site
func (h *SiteHandler) Update(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req dto.UpdateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	site, err := h.siteService.Update(c.Context(), siteID, serverID, userID, &req)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Site updated", dto.ToSiteResponse(site))
}

// Delete deletes a site
func (h *SiteHandler) Delete(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.siteService.Delete(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Site deletion initiated", nil)
}

// GetDeletionSummary returns a summary of resources to be deleted
func (h *SiteHandler) GetDeletionSummary(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	summary, err := h.siteService.GetDeletionSummary(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.InternalError(c, "Failed to get deletion summary")
	}

	return response.OK(c, "Deletion summary retrieved", summary)
}

// RegenerateDeployToken regenerates the deploy token
func (h *SiteHandler) RegenerateDeployToken(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	site, err := h.siteService.RegenerateDeployToken(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Deploy token regenerated", dto.ToSiteResponse(site))
}

// UpdateDeploymentSettings updates deployment settings
func (h *SiteHandler) UpdateDeploymentSettings(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req dto.UpdateDeploymentSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	// Convert to UpdateSiteRequest
	updateReq := &dto.UpdateSiteRequest{
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

	site, err := h.siteService.Update(c.Context(), siteID, serverID, userID, updateReq)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Deployment settings updated", dto.ToSiteResponse(site))
}

// GetSettings returns the site settings page data
func (h *SiteHandler) GetSettings(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	settingsData, err := h.siteService.GetSettings(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.InternalError(c, "Failed to fetch site settings")
	}

	// Build TLS options
	tlsOptions := make([]dto.TlsOptionResponse, 0)
	for _, tls := range enums.AllTlsSettings() {
		tlsOptions = append(tlsOptions, dto.TlsOptionResponse{
			Value: string(tls),
			Label: tls.Label(),
		})
	}

	resp := dto.SiteSettingsResponse{
		Site:          dto.ToSiteResponse(settingsData.Site),
		TlsOptions:    tlsOptions,
		PhpVersions:   settingsData.PhpVersions,
		SourceControl: settingsData.SourceControl,
		Repository:    settingsData.Repository,
	}

	if settingsData.ActiveCertificate != nil {
		certResp := dto.ToCertificateResponse(settingsData.ActiveCertificate)
		resp.ActiveCertificate = &certResp
	}

	return response.OK(c, "Site settings retrieved", resp)
}
