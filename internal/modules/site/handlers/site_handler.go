package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// SiteHandler holds the site endpoints that do not fit a generic route
// helper. Standard CRUD on /servers/:serverId/sites is wired directly
// to the framework helpers in routes.go.
type SiteHandler struct {
	siteService *services.SiteService
	domainRepo  dnscontracts.DomainRepository
}

// NewSiteHandler creates a new site handler.
func NewSiteHandler(siteService *services.SiteService) *SiteHandler {
	return &SiteHandler{siteService: siteService}
}

// SetDomainRepository sets the domain repository for cross-module queries.
func (h *SiteHandler) SetDomainRepository(repo dnscontracts.DomainRepository) {
	h.domainRepo = repo
}

// Show returns a single site enriched with queue count and source-control
// info — composes 3 service calls so it stays bespoke.
func (h *SiteHandler) Show(r *fiberctx.Request) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(r.Ctx)
	if err != nil {
		return err
	}

	site, err := h.siteService.FindByID(r.Context(), siteID, serverID, r.TeamID)
	if err != nil {
		return err
	}

	resp := dto.ToSiteResponse(site)
	resp.QueueCount = h.siteService.GetQueueCount(r.Context(), site.ID)

	if site.SourceControlID != nil && *site.SourceControlID != "" {
		resp.SourceControl, resp.Repository = h.siteService.GetSourceControlInfo(
			r.Context(),
			r.TeamID,
			*site.SourceControlID,
			site.SourceControlRepositoriesID,
		)
	}
	return fiberctx.OK(r.Ctx, "Site retrieved", resp)
}

// Delete kicks off async site deletion. Returns 200 with a status
// message rather than 204 because the resource is not gone yet — the
// generic Delete helper would change the status code.
func (h *SiteHandler) Delete(r *fiberctx.Request) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(r.Ctx)
	if err != nil {
		return err
	}
	if err := h.siteService.Delete(r.Context(), siteID, serverID, r.TeamID); err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "Site deletion initiated", nil)
}

// RegenerateDeployToken regenerates the deploy token. POST without body
// that returns the updated site DTO — does not fit ActionItem (no
// payload) or Update (no body).
func (h *SiteHandler) RegenerateDeployToken(r *fiberctx.Request) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(r.Ctx)
	if err != nil {
		return err
	}
	site, err := h.siteService.RegenerateDeployToken(r.Context(), siteID, serverID, r.TeamID)
	if err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "Deploy token regenerated", dto.ToSiteResponse(site))
}

// UpdateDeploymentSettings updates a subset of site fields. PATCH that
// reuses Update with a coerced request — stays bespoke for the
// translation step.
func (h *SiteHandler) UpdateDeploymentSettings(r *fiberctx.Request, req *dto.UpdateDeploymentSettingsRequest) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(r.Ctx)
	if err != nil {
		return err
	}

	updateReq := &dto.UpdateSiteRequest{
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

	site, err := h.siteService.Update(r.Context(), siteID, serverID, r.TeamID, r.UserID, updateReq)
	if err != nil {
		return err
	}
	return fiberctx.OK(r.Ctx, "Deployment settings updated", site)
}

// GetSettings returns a composite payload combining site, TLS options,
// PHP versions, source-control + repository info, and the active
// certificate.
func (h *SiteHandler) GetSettings(r *fiberctx.Request) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(r.Ctx)
	if err != nil {
		return err
	}

	settingsData, err := h.siteService.GetSettings(r.Context(), siteID, serverID, r.TeamID)
	if err != nil {
		return err
	}

	tlsOptions := make([]dto.TLSOptionResponse, 0, len(sitetypes.AllTLSSettings()))
	for _, tls := range sitetypes.AllTLSSettings() {
		tlsOptions = append(tlsOptions, dto.TLSOptionResponse{Value: string(tls), Label: tls.Label()})
	}

	resp := dto.SiteSettingsResponse{
		Site:          dto.ToSiteResponse(settingsData.Site),
		TLSOptions:    tlsOptions,
		PhpVersions:   settingsData.PhpVersions,
		SourceControl: settingsData.SourceControl,
		Repository:    settingsData.Repository,
	}
	if settingsData.ActiveCertificate != nil {
		certResp := dto.ToCertificateResponse(settingsData.ActiveCertificate)
		resp.ActiveCertificate = &certResp
	}
	return fiberctx.OK(r.Ctx, "Site settings retrieved", resp)
}

// GetCreateOptions returns the static options for creating a site.
// Top-level (no server scope), so it does not fit Index.
func (h *SiteHandler) GetCreateOptions(c *fiber.Ctx) error {
	return fiberctx.OK(c, "Create options retrieved", dto.GetCreateSiteOptions())
}

// VerifyDomain checks if a domain is connected to the user's team. Has
// a query-string `domain` parameter so it does not fit any helper.
func (h *SiteHandler) VerifyDomain(c *fiber.Ctx) error {
	domain := c.Query("domain")
	if domain == "" {
		return fiberctx.RespondBadRequest(c, fiberctx.MsgMissingRequiredParams)
	}
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	baseDomain := getBaseDomain(domain)

	if h.domainRepo == nil {
		return fiberctx.OK(c, "Domain verification result", dto.VerifyDomainResponse{
			Verified: false, Domain: domain, BaseDomain: baseDomain, CanCreateRecord: false,
		})
	}

	userDomains, err := h.domainRepo.FindByTeam(c.Context(), teamID)
	if err != nil {
		return err
	}
	for _, userDomain := range userDomains {
		if userDomain.Address == baseDomain {
			return fiberctx.OK(c, "Domain verification result", dto.VerifyDomainResponse{
				Verified: true, Domain: domain, BaseDomain: baseDomain,
				ConnectedDomainID: &userDomain.ID, CanCreateRecord: true,
			})
		}
	}
	return fiberctx.OK(c, "Domain verification result", dto.VerifyDomainResponse{
		Verified: false, Domain: domain, BaseDomain: baseDomain, CanCreateRecord: false,
	})
}

// getBaseDomain extracts the base domain from a full domain
// (e.g. "demo3.gig.code" -> "gig.code").
func getBaseDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}
	return strings.Join(parts[len(parts)-2:], ".")
}
