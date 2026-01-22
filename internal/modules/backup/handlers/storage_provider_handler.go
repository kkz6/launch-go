package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/enums"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// StorageProviderHandler handles HTTP requests for storage providers
type StorageProviderHandler struct {
	providerService *services.StorageProviderService
}

// NewStorageProviderHandler creates a new storage provider handler
func NewStorageProviderHandler(providerService *services.StorageProviderService) *StorageProviderHandler {
	return &StorageProviderHandler{providerService: providerService}
}

// ListStorageProviders lists all storage providers for a team
func (h *StorageProviderHandler) ListStorageProviders(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	providers, err := h.providerService.ListStorageProvidersByTeam(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, response.MsgInternalError)
	}

	result := make([]dto.StorageProviderResponse, len(providers))
	for i, provider := range providers {
		result[i] = dto.ToStorageProviderResponse(&provider)
	}

	return response.OK(c, "Storage providers retrieved", result)
}

// ListStorageProvidersForDropdown returns a simplified list for dropdowns
func (h *StorageProviderHandler) ListStorageProvidersForDropdown(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	providers, err := h.providerService.ListStorageProvidersByTeam(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, response.MsgInternalError)
	}

	result := make(map[uint64]string)
	for _, provider := range providers {
		label := ""
		if provider.Label != nil {
			label = *provider.Label
		}
		result[provider.ID] = label
	}

	return response.OK(c, "Storage providers retrieved", result)
}

// ConnectStorageProvider creates a new storage provider connection
func (h *StorageProviderHandler) ConnectStorageProvider(c *fiber.Ctx) error {
	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}
	providerType := c.Params("provider")

	// Validate provider type
	driver := enums.StorageDriver(providerType)
	if !driver.IsValid() {
		return response.BadRequest(c, "Invalid storage provider type")
	}

	req, err := fiberutil.MustParseAndValidate[dto.CreateStorageProviderRequest](c)
	if err != nil {
		return err
	}

	// Set provider from URL param
	req.Provider = providerType

	provider, err := h.providerService.ConnectStorageProvider(c.Context(), userID, teamID, req)
	if err != nil {
		if err == services.ErrConnectionFailed {
			return response.BadRequest(c, "Failed to connect to storage provider")
		}
		return response.HandleError(c, err)
	}

	return response.Created(c, "Storage provider connected successfully", dto.ToStorageProviderResponse(provider))
}

// UpdateStorageProvider updates an existing storage provider
func (h *StorageProviderHandler) UpdateStorageProvider(c *fiber.Ctx) error {
	providerType := c.Params("provider")

	// Validate provider type
	driver := enums.StorageDriver(providerType)
	if !driver.IsValid() {
		return response.BadRequest(c, "Invalid storage provider type")
	}

	req, err := fiberutil.MustParseAndValidate[dto.UpdateStorageProviderRequest](c)
	if err != nil {
		return err
	}

	// Set provider from URL param
	req.Provider = providerType

	provider, err := h.providerService.UpdateStorageProvider(c.Context(), req.ID, req)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgStorageProviderNotFound)
		}
		if err == services.ErrConnectionFailed {
			return response.BadRequest(c, "Failed to connect to storage provider")
		}
		return response.HandleError(c, err)
	}

	return response.OK(c, "Storage provider updated successfully", dto.ToStorageProviderResponse(provider))
}

// DeleteStorageProvider deletes a storage provider
func (h *StorageProviderHandler) DeleteStorageProvider(c *fiber.Ctx) error {
	providerIDStr := c.Params("provider")

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		return response.BadRequest(c, "Invalid provider ID")
	}

	if err := h.providerService.DeleteStorageProvider(c.Context(), providerID); err != nil {
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgStorageProviderNotFound)
		}
		if err == services.ErrStorageProviderHasBackups {
			return response.Conflict(c, "Storage provider has associated backups and cannot be deleted")
		}
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// ShowStorageProvider shows a single storage provider
func (h *StorageProviderHandler) ShowStorageProvider(c *fiber.Ctx) error {
	providerIDStr := c.Params("id")

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		return response.BadRequest(c, "Invalid provider ID")
	}

	provider, err := h.providerService.GetStorageProvider(c.Context(), providerID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgStorageProviderNotFound)
		}
		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Storage provider retrieved", dto.ToStorageProviderResponse(provider))
}
