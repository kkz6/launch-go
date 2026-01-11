package dns

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for the DNS module
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler instance
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Domain Provider Handlers

// ListProviders lists all DNS providers for the current team
func (h *Handler) ListProviders(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	providers, err := h.service.ListProviders(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch providers")
	}

	return response.OK(c, "Providers retrieved", providers)
}

// CreateProvider creates a new DNS provider
func (h *Handler) CreateProvider(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)

	var req CreateDomainProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	provider, err := h.service.CreateProvider(c.Context(), userID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return response.Error(c, fiber.StatusBadRequest, "Invalid credentials. Please check your API token and try again.")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	count, _ := h.service.repo.CountDomainsByProvider(c.Context(), provider.ID)
	return response.Created(c, "Provider created", ToDomainProviderResponse(provider, int(count)))
}

// DeleteProvider deletes a DNS provider
func (h *Handler) DeleteProvider(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	err := h.service.DeleteProvider(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, ErrProviderNotFound) {
			return response.NotFound(c, "Provider not found")
		}
		if errors.Is(err, ErrProviderHasActiveDomains) {
			return response.Error(c, fiber.StatusBadRequest, "Cannot delete provider with active domains")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// CheckProviderConnectivity checks if provider credentials are valid
func (h *Handler) CheckProviderConnectivity(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	err := h.service.CheckProviderConnectivity(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, ErrProviderNotFound) {
			return response.NotFound(c, "Provider not found")
		}
		return response.Error(c, fiber.StatusBadRequest, "Provider connectivity check failed")
	}

	return response.OK(c, "Provider is connected", nil)
}

// SyncProviderDomains syncs domains from a provider
func (h *Handler) SyncProviderDomains(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	err := h.service.SyncDomains(c.Context(), id, userID, teamID)
	if err != nil {
		if errors.Is(err, ErrProviderNotFound) {
			return response.NotFound(c, "Provider not found")
		}
		return response.Error(c, fiber.StatusBadRequest, "Failed to sync domains")
	}

	return response.OK(c, "Domains synchronized successfully", nil)
}

// Domain Handlers

// ListDomains lists all domains for the current team
func (h *Handler) ListDomains(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	domains, err := h.service.ListDomains(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch domains")
	}

	// Also get providers for the dropdown
	providers, _ := h.service.ListProviders(c.Context(), teamID)

	return response.OK(c, "Domains retrieved", fiber.Map{
		"domains":   domains,
		"providers": providers,
	})
}

// CreateDomain creates a new domain
func (h *Handler) CreateDomain(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)

	var req CreateDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	domain, err := h.service.CreateDomain(c.Context(), userID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrProviderNotFound) {
			return response.NotFound(c, "Provider not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Domain created", ToDomainResponse(domain))
}

// ShowDomain retrieves a domain by ID
func (h *Handler) ShowDomain(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	domain, err := h.service.GetDomain(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	recordTypes := h.service.GetRecordTypes()

	return response.OK(c, "Domain retrieved", fiber.Map{
		"domain":       ToDomainResponse(domain),
		"record_types": recordTypes,
	})
}

// DeleteDomain deletes a domain
func (h *Handler) DeleteDomain(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	var req DeleteDomainRequest
	c.BodyParser(&req) // Optional body, defaults to false

	err := h.service.DeleteDomain(c.Context(), id, teamID, req.DeleteFromProvider)
	if err != nil {
		if errors.Is(err, ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// DNS Record Handlers

// ListRecords lists all DNS records for a domain
func (h *Handler) ListRecords(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	domainID := c.Params("id")

	records, err := h.service.GetDomainRecords(c.Context(), domainID, teamID)
	if err != nil {
		if errors.Is(err, ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Records retrieved", records)
}

// CreateRecord creates a new DNS record
func (h *Handler) CreateRecord(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	domainID := c.Params("id")

	var req CreateDnsRecordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	record, err := h.service.CreateRecord(c.Context(), domainID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Record created", ToDnsRecordResponse(record))
}

// UpdateRecord updates a DNS record
func (h *Handler) UpdateRecord(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	domainID := c.Params("domainId")
	recordID := c.Params("recordId")

	var req UpdateDnsRecordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	record, err := h.service.UpdateRecord(c.Context(), recordID, domainID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		if errors.Is(err, ErrRecordNotFound) {
			return response.NotFound(c, "Record not found")
		}
		if errors.Is(err, ErrRecordNotEditable) {
			return response.Error(c, fiber.StatusForbidden, "This record type cannot be edited")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Record updated", ToDnsRecordResponse(record))
}

// DeleteRecord deletes a DNS record
func (h *Handler) DeleteRecord(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	domainID := c.Params("domainId")
	recordID := c.Params("recordId")

	err := h.service.DeleteRecord(c.Context(), recordID, domainID, teamID)
	if err != nil {
		if errors.Is(err, ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		if errors.Is(err, ErrRecordNotFound) {
			return response.NotFound(c, "Record not found")
		}
		if errors.Is(err, ErrRecordNotDeletable) {
			return response.Error(c, fiber.StatusForbidden, "This record type cannot be deleted")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// GetRecordTypes returns all available record types
func (h *Handler) GetRecordTypes(c *fiber.Ctx) error {
	types := h.service.GetRecordTypes()
	return response.OK(c, "Record types retrieved", types)
}
