package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// StorageProviderHandler holds the storage provider endpoints that do
// not fit the team-scoped route helpers. These routes use the :provider
// path parameter for two different things:
//
//   - POST /storage-providers/:provider/connect — :provider is the type
//     ("s3", "dropbox") and is copied into the request body.
//   - PUT /storage-providers/:provider          — :provider is the type;
//     the actual provider id is in the body.
//   - DELETE /storage-providers/:provider       — :provider is the
//     numeric (uint64) id.
//
// The list / dropdown / show endpoints are wired directly to route
// helpers in routes.go.
type StorageProviderHandler struct {
	providerService *services.StorageProviderService
}

// NewStorageProviderHandler creates a new storage provider handler.
func NewStorageProviderHandler(providerService *services.StorageProviderService) *StorageProviderHandler {
	return &StorageProviderHandler{providerService: providerService}
}

// ConnectStorageProvider creates a new storage provider connection.
func (h *StorageProviderHandler) ConnectStorageProvider(r *fiberutil.Request) error {
	providerType := r.Params("provider")

	if !backuptypes.StorageDriver(providerType).IsValid() {
		return fiberutil.BadRequest("Invalid storage provider type")
	}

	req, err := fiberutil.MustParseAndValidate[dto.CreateStorageProviderRequest](r.Ctx)
	if err != nil {
		return err
	}
	req.Provider = providerType

	resp, err := h.providerService.ConnectStorageProvider(r.Context(), r.TeamID, r.UserID, req)
	if err != nil {
		return err
	}
	return fiberutil.Created(r.Ctx, "Storage provider connected successfully", resp)
}

// UpdateStorageProvider updates an existing storage provider. The path
// parameter `:provider` carries the provider type; the actual provider
// id is in the request body.
func (h *StorageProviderHandler) UpdateStorageProvider(c *fiber.Ctx) error {
	providerType := c.Params("provider")
	if !backuptypes.StorageDriver(providerType).IsValid() {
		return fiberutil.BadRequest("Invalid storage provider type")
	}

	req, err := fiberutil.MustParseAndValidate[dto.UpdateStorageProviderRequest](c)
	if err != nil {
		return err
	}
	req.Provider = providerType

	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	resp, err := h.providerService.UpdateStorageProvider(c.Context(), req.ID, teamID, req)
	if err != nil {
		return err
	}
	return fiberutil.OK(c, "Storage provider updated successfully", resp)
}

// DeleteStorageProvider deletes a storage provider by numeric id.
func (h *StorageProviderHandler) DeleteStorageProvider(c *fiber.Ctx) error {
	providerID, err := strconv.ParseUint(c.Params("provider"), 10, 64)
	if err != nil {
		return fiberutil.BadRequest("Invalid provider ID")
	}
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	if err := h.providerService.DeleteStorageProvider(c.Context(), providerID, teamID); err != nil {
		return err
	}
	return fiberutil.NoContent(c)
}
