package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
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
	teamID := c.Locals("teamID").(string)

	domains, err := h.domainService.ListDomains(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch domains")
	}

	// Also get providers for the dropdown
	providers, _ := h.providerService.ListProviders(c.Context(), teamID)

	pageData := dto.DomainIndexPageData{
		Domains:   domains,
		Providers: providers,
	}

	return response.OK(c, "Domains retrieved", pageData)
}

// CreateDomain creates a new domain
func (h *DomainHandler) CreateDomain(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)

	var req dto.CreateDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	domain, err := h.domainService.CreateDomain(c.Context(), userID, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrProviderNotFound) {
			return response.NotFound(c, "Provider not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Domain created", dto.ToDomainResponse(domain))
}

// ShowDomain retrieves a domain by ID
func (h *DomainHandler) ShowDomain(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	domain, err := h.domainService.GetDomain(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	// Get records for this domain
	records, _ := h.domainService.GetDomainRecords(c.Context(), id, teamID)

	// Get record types
	recordService := services.NewDnsRecordService(nil, nil, nil)
	recordTypesList := recordService.GetRecordTypes()

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

	return response.OK(c, "Domain retrieved", pageData)
}

// UpdateDomain updates a domain
func (h *DomainHandler) UpdateDomain(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	var req dto.UpdateDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	domain, err := h.domainService.UpdateDomain(c.Context(), id, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Domain updated", dto.ToDomainResponse(domain))
}

// DeleteDomain deletes a domain
func (h *DomainHandler) DeleteDomain(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	var req dto.DeleteDomainRequest
	c.BodyParser(&req) // Optional body, defaults to false

	err := h.domainService.DeleteDomain(c.Context(), id, teamID, req.DeleteFromProvider)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// SyncDomain syncs DNS records from the provider to the local database
func (h *DomainHandler) SyncDomain(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	err := h.domainService.SyncDomainRecords(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "DNS records synced successfully", nil)
}
