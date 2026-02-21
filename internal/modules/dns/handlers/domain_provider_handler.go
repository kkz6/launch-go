package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	providers, err := h.providerService.ListProviders(c.Context(), teamID)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Providers retrieved", providers)
}

// CreateProvider creates a new DNS provider
func (h *DomainProviderHandler) CreateProvider(c *fiber.Ctx) error {
	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberutil.MustParseAndValidate[dto.CreateDomainProviderRequest](c)
	if err != nil {
		return err
	}

	provider, err := h.providerService.CreateProvider(c.Context(), userID, teamID, req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			return fiberutil.RespondBadRequest(c, fiberutil.MsgInvalidCredentials)
		}
		return fiberutil.HandleError(c, err)
	}

	// Count is 0 for newly created provider, but we check anyway
	count, err := h.providerService.CountDomainsByProvider(c.Context(), provider.ID)
	if err != nil {
		// Non-critical error - provider was created, just use 0 for count
		count = 0
	}

	return fiberutil.Created(c, "Provider created", dto.ToDomainProviderResponse(provider, int(count)))
}

// DeleteProvider deletes a DNS provider
func (h *DomainProviderHandler) DeleteProvider(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	err = h.providerService.DeleteProvider(c.Context(), id, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Provider not found")
		}
		if errors.Is(err, services.ErrProviderHasActiveDomains) {
			return fiberutil.RespondBadRequest(c, "Cannot delete provider with active domains")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.NoContent(c)
}

// CheckProviderConnectivity checks if provider credentials are valid
func (h *DomainProviderHandler) CheckProviderConnectivity(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	err = h.providerService.CheckProviderConnectivity(c.Context(), id, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Provider not found")
		}
		return fiberutil.RespondBadRequest(c, "Provider connectivity check failed")
	}

	return fiberutil.OK(c, "Provider is connected", nil)
}

// SyncProviderDomains syncs domains from a provider
func (h *DomainProviderHandler) SyncProviderDomains(c *fiber.Ctx) error {
	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}
	id := c.Params("id")

	err = h.providerService.SyncDomains(c.Context(), id, userID, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Provider not found")
		}
		return fiberutil.RespondBadRequest(c, "Failed to sync domains")
	}

	return fiberutil.OK(c, "Domains synchronized successfully", nil)
}
