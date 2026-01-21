package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// SiteHandler handles HTTP requests for sites
type SiteHandler struct {
	siteService *services.SiteService
	domainRepo  dnscontracts.DomainRepository
}

// NewSiteHandler creates a new site handler
func NewSiteHandler(siteService *services.SiteService) *SiteHandler {
	return &SiteHandler{siteService: siteService}
}

// SetDomainRepository sets the domain repository for cross-module queries
func (h *SiteHandler) SetDomainRepository(repo dnscontracts.DomainRepository) {
	h.domainRepo = repo
}

// List returns all sites for a server
func (h *SiteHandler) List(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	sites, err := h.siteService.List(c.Context(), serverID, teamID)
	if err != nil {
		return response.InternalError(c, response.MsgInternalError)
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
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateSiteRequest](c)
	if err != nil {
		return err
	}

	site, err := h.siteService.Create(c.Context(), serverID, teamID, userID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Site created", dto.ToSiteResponse(site))
}

// Show returns a single site
func (h *SiteHandler) Show(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	site, err := h.siteService.FindByID(c.Context(), siteID, serverID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	resp := dto.ToSiteResponse(site)

	// Include source control and repository info if linked
	if site.SourceControlID != nil && *site.SourceControlID != "" {
		resp.SourceControl, resp.Repository = h.siteService.GetSourceControlInfo(
			c.Context(),
			*site.SourceControlID,
			site.SourceControlRepositoriesID,
		)
	}

	return response.OK(c, "Site retrieved", resp)
}

// Update updates a site
func (h *SiteHandler) Update(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateSiteRequest](c)
	if err != nil {
		return err
	}

	site, err := h.siteService.Update(c.Context(), siteID, serverID, teamID, userID, req)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Site updated", dto.ToSiteResponse(site))
}

// Delete deletes a site
func (h *SiteHandler) Delete(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	if err := h.siteService.Delete(c.Context(), siteID, serverID, teamID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Site deletion initiated", nil)
}

// GetDeletionSummary returns a summary of resources to be deleted
func (h *SiteHandler) GetDeletionSummary(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	summary, err := h.siteService.GetDeletionSummary(c.Context(), siteID, serverID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Deletion summary retrieved", summary)
}

// RegenerateDeployToken regenerates the deploy token
func (h *SiteHandler) RegenerateDeployToken(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	site, err := h.siteService.RegenerateDeployToken(c.Context(), siteID, serverID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Deploy token regenerated", dto.ToSiteResponse(site))
}

// UpdateDeploymentSettings updates deployment settings
func (h *SiteHandler) UpdateDeploymentSettings(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateDeploymentSettingsRequest](c)
	if err != nil {
		return err
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

	site, err := h.siteService.Update(c.Context(), siteID, serverID, teamID, userID, updateReq)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Deployment settings updated", dto.ToSiteResponse(site))
}

// GetSettings returns the site settings page data
func (h *SiteHandler) GetSettings(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	settingsData, err := h.siteService.GetSettings(c.Context(), siteID, serverID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.InternalError(c, response.MsgInternalError)
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

// VerifyDomain checks if a domain is connected to the user's team
func (h *SiteHandler) VerifyDomain(c *fiber.Ctx) error {
	domain := c.Query("domain")
	if domain == "" {
		return response.BadRequest(c, response.MsgMissingRequiredParams)
	}

	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	// Extract the base domain (handles subdomains)
	baseDomain := getBaseDomain(domain)

	// Check if domain repository is configured
	if h.domainRepo == nil {
		// No domain repository configured, return not verified
		return response.OK(c, "Domain verification result", dto.VerifyDomainResponse{
			Verified:        false,
			Domain:          domain,
			BaseDomain:      baseDomain,
			CanCreateRecord: false,
		})
	}

	// Get user's connected domains
	userDomains, err := h.domainRepo.FindByTeam(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, response.MsgInternalError)
	}

	// Check if the base domain exists in user's connected domains
	for _, userDomain := range userDomains {
		if userDomain.Address == baseDomain {
			return response.OK(c, "Domain verification result", dto.VerifyDomainResponse{
				Verified:          true,
				Domain:            domain,
				BaseDomain:        baseDomain,
				ConnectedDomainID: &userDomain.ID,
				CanCreateRecord:   true,
			})
		}
	}

	return response.OK(c, "Domain verification result", dto.VerifyDomainResponse{
		Verified:        false,
		Domain:          domain,
		BaseDomain:      baseDomain,
		CanCreateRecord: false,
	})
}

// getBaseDomain extracts the base domain from a full domain (handles subdomains)
// e.g., "demo3.gig.code" -> "gig.code"
func getBaseDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}

	return strings.Join(parts[len(parts)-2:], ".")
}
