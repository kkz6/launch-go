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

	return response.OK(c, "Domains retrieved", fiber.Map{
		"domains":   domains,
		"providers": providers,
	})
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

	recordService := services.NewDnsRecordService(nil, nil, nil)
	recordTypes := recordService.GetRecordTypes()

	return response.OK(c, "Domain retrieved", fiber.Map{
		"domain":       dto.ToDomainResponse(domain),
		"record_types": recordTypes,
	})
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
