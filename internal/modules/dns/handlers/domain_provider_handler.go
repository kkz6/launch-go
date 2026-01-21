package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// DomainProviderHandler handles HTTP requests for domain providers
type DomainProviderHandler struct {
	providerService *services.DomainProviderService
}

// NewDomainProviderHandler creates a new DomainProviderHandler instance
func NewDomainProviderHandler(providerService *services.DomainProviderService) *DomainProviderHandler {
	return &DomainProviderHandler{providerService: providerService}
}

// ListProviders lists all DNS providers for the current team
func (h *DomainProviderHandler) ListProviders(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	providers, err := h.providerService.ListProviders(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch providers")
	}

	return response.OK(c, "Providers retrieved", providers)
}

// CreateProvider creates a new DNS provider
func (h *DomainProviderHandler) CreateProvider(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)

	var req dto.CreateDomainProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	provider, err := h.providerService.CreateProvider(c.Context(), userID, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			return response.Error(c, fiber.StatusBadRequest, "Invalid credentials. Please check your API token and try again.")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	// Count is 0 for newly created provider, but we check anyway
	count, err := h.providerService.CountDomainsByProvider(c.Context(), provider.ID)
	if err != nil {
		// Non-critical error - provider was created, just use 0 for count
		count = 0
	}

	return response.Created(c, "Provider created", dto.ToDomainProviderResponse(provider, int(count)))
}

// DeleteProvider deletes a DNS provider
func (h *DomainProviderHandler) DeleteProvider(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	err := h.providerService.DeleteProvider(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, services.ErrProviderNotFound) {
			return response.NotFound(c, "Provider not found")
		}
		if errors.Is(err, services.ErrProviderHasActiveDomains) {
			return response.Error(c, fiber.StatusBadRequest, "Cannot delete provider with active domains")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// CheckProviderConnectivity checks if provider credentials are valid
func (h *DomainProviderHandler) CheckProviderConnectivity(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	id := c.Params("id")

	err := h.providerService.CheckProviderConnectivity(c.Context(), id, teamID)
	if err != nil {
		if errors.Is(err, services.ErrProviderNotFound) {
			return response.NotFound(c, "Provider not found")
		}
		return response.Error(c, fiber.StatusBadRequest, "Provider connectivity check failed")
	}

	return response.OK(c, "Provider is connected", nil)
}

// SyncProviderDomains syncs domains from a provider
func (h *DomainProviderHandler) SyncProviderDomains(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)
	id := c.Params("id")

	err := h.providerService.SyncDomains(c.Context(), id, userID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrProviderNotFound) {
			return response.NotFound(c, "Provider not found")
		}
		return response.Error(c, fiber.StatusBadRequest, "Failed to sync domains")
	}

	return response.OK(c, "Domains synchronized successfully", nil)
}
