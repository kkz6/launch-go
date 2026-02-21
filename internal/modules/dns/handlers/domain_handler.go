package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DomainHandler handles HTTP requests for domains
type DomainHandler struct {
	domainService   *services.DomainService
	providerService *services.DomainProviderService
}

// NewDomainHandler creates a new DomainHandler instance
func NewDomainHandler(domainService *services.DomainService, providerService *services.DomainProviderService) *DomainHandler {
	return &DomainHandler{
		domainService:   domainService,
		providerService: providerService,
	}
}

// ListDomains lists all domains for the current team
func (h *DomainHandler) ListDomains(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	domains, err := h.domainService.ListDomains(c.Context(), teamID)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	// Also get providers for the dropdown
	providers, err := h.providerService.ListProviders(c.Context(), teamID)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	pageData := dto.DomainIndexPageData{
		Domains:   domains,
		Providers: providers,
	}

	return fiberutil.OK(c, "Domains retrieved", pageData)
}

// CreateDomain creates a new domain
func (h *DomainHandler) CreateDomain(c *fiber.Ctx) error {
	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberutil.MustParseAndValidate[dto.CreateDomainRequest](c)
	if err != nil {
		return err
	}

	domain, err := h.domainService.CreateDomain(c.Context(), userID, teamID, req)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Provider not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.Created(c, "Domain created", dto.ToDomainResponse(domain))
}

// ShowDomain retrieves a domain by ID
func (h *DomainHandler) ShowDomain(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	domain, err := h.domainService.GetDomain(c.Context(), id, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Domain not found")
		}
		return fiberutil.HandleError(c, err)
	}

	// Get records for this domain
	records, err := h.domainService.GetDomainRecords(c.Context(), id, teamID)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	// Get record types
	recordTypesList := services.GetRecordTypes()

	// Convert record types to structured format
	recordTypes := make([]dto.RecordTypeOption, len(recordTypesList))
	for i, rt := range recordTypesList {
		recordTypes[i] = dto.RecordTypeOption{
			Value: rt,
			Label: rt,
		}
	}

	// Extract nameservers from NS records (already in database)
	nameservers := make([]string, 0)
	for _, record := range records {
		if record.Type == "NS" {
			nameservers = append(nameservers, record.Value)
		}
	}

	// Get provider details if available
	var providerResponse *dto.DomainProviderResponse
	if domain.Provider != nil {
		pr := dto.ToDomainProviderResponse(domain.Provider, 0)
		providerResponse = &pr
	}

	pageData := dto.DomainShowPageData{
		Domain:      dto.ToDomainResponse(domain),
		Records:     records,
		RecordTypes: recordTypes,
		Nameservers: nameservers,
		Provider:    providerResponse,
	}

	return fiberutil.OK(c, "Domain retrieved", pageData)
}

// UpdateDomain updates a domain
func (h *DomainHandler) UpdateDomain(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	req, err := fiberutil.MustParseAndValidate[dto.UpdateDomainRequest](c)
	if err != nil {
		return err
	}

	domain, err := h.domainService.UpdateDomain(c.Context(), id, teamID, req)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Domain not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Domain updated", dto.ToDomainResponse(domain))
}

// DeleteDomain deletes a domain
func (h *DomainHandler) DeleteDomain(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	var req dto.DeleteDomainRequest
	c.BodyParser(&req) // Optional body, defaults to false

	err = h.domainService.DeleteDomain(c.Context(), id, teamID, req.DeleteFromProvider)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Domain not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.NoContent(c)
}

// SyncDomain syncs DNS records from the provider to the local database
func (h *DomainHandler) SyncDomain(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	err = h.domainService.SyncDomainRecords(c.Context(), id, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Domain not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "DNS records synced successfully", nil)
}
