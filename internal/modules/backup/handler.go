package backup

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for the backup module
type Handler struct {
	service *Service
}

// NewHandler creates a new backup handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Backup endpoints

// ListBackups lists all backups for a server
func (h *Handler) ListBackups(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	backups, err := h.service.ListBackupsByServer(c.Context(), serverID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch backups")
	}

	result := make([]BackupResponse, len(backups))
	for i, backup := range backups {
		result[i] = ToBackupResponse(&backup)
	}

	return response.OK(c, "Backups retrieved", result)
}

// CreateBackup creates a new backup configuration
func (h *Handler) CreateBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	userID := c.Locals("userID").(string)

	var req CreateBackupRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	backup, err := h.service.CreateBackup(c.Context(), serverID, userID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Backup created successfully", ToBackupResponse(backup))
}

// ShowBackup shows a single backup
func (h *Handler) ShowBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	backup, err := h.service.GetBackupByIDAndServer(c.Context(), backupID, serverID)
	if err != nil {
		if err == ErrBackupNotFound {
			return response.NotFound(c, "Backup not found")
		}
		return response.InternalError(c, "Failed to fetch backup")
	}

	return response.OK(c, "Backup retrieved", ToBackupResponse(backup))
}

// UpdateBackup updates a backup configuration
func (h *Handler) UpdateBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	// Verify backup belongs to server
	_, err := h.service.GetBackupByIDAndServer(c.Context(), backupID, serverID)
	if err != nil {
		if err == ErrBackupNotFound {
			return response.NotFound(c, "Backup not found")
		}
		return response.InternalError(c, "Failed to fetch backup")
	}

	var req UpdateBackupRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	backup, err := h.service.UpdateBackup(c.Context(), backupID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Backup updated successfully", ToBackupResponse(backup))
}

// DeleteBackup deletes a backup configuration
func (h *Handler) DeleteBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	if err := h.service.DeleteBackup(c.Context(), backupID, serverID); err != nil {
		if err == ErrBackupNotFound {
			return response.NotFound(c, "Backup not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// RunManualBackup triggers a manual backup run
func (h *Handler) RunManualBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	if err := h.service.RunBackup(c.Context(), backupID, serverID); err != nil {
		if err == ErrBackupNotFound {
			return response.NotFound(c, "Backup not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Backup queued for execution", nil)
}

// BackupJob endpoints

// CreateBackupJob creates a new backup job (webhook endpoint for agent)
func (h *Handler) CreateBackupJob(c *fiber.Ctx) error {
	backupID := c.Params("backup")
	token := c.Params("token")

	var req CreateBackupJobRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	_, err := h.service.CreateBackupJob(c.Context(), backupID, token, &req)
	if err != nil {
		if err.Error() == "invalid dispatch token" {
			return response.Forbidden(c, "Invalid dispatch token")
		}
		if err == ErrBackupNotFound {
			return response.NotFound(c, "Backup not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// StorageProvider endpoints

// ListStorageProviders lists all storage providers for a team
func (h *Handler) ListStorageProviders(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	providers, err := h.service.ListStorageProvidersByTeam(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch storage providers")
	}

	result := make([]StorageProviderResponse, len(providers))
	for i, provider := range providers {
		result[i] = ToStorageProviderResponse(&provider)
	}

	return response.OK(c, "Storage providers retrieved", result)
}

// ListStorageProvidersForDropdown returns a simplified list for dropdowns
func (h *Handler) ListStorageProvidersForDropdown(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	providers, err := h.service.ListStorageProvidersByTeam(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch storage providers")
	}

	result := make(map[uint]string)
	for _, provider := range providers {
		result[provider.ID] = provider.Label
	}

	return response.OK(c, "Storage providers retrieved", result)
}

// ConnectStorageProvider creates a new storage provider connection
func (h *Handler) ConnectStorageProvider(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	providerType := c.Params("provider")

	// Validate provider type
	driver := StorageDriver(providerType)
	if !driver.IsValid() {
		return response.Error(c, fiber.StatusBadRequest, "Invalid storage provider type")
	}

	var req CreateStorageProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// Set provider from URL param
	req.Provider = providerType

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	provider, err := h.service.ConnectStorageProvider(c.Context(), userID, teamID, &req)
	if err != nil {
		if err == ErrConnectionFailed {
			return response.Error(c, fiber.StatusBadRequest, "Failed to connect to storage provider")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Storage provider connected successfully", ToStorageProviderResponse(provider))
}

// UpdateStorageProvider updates an existing storage provider
func (h *Handler) UpdateStorageProvider(c *fiber.Ctx) error {
	providerType := c.Params("provider")

	// Validate provider type
	driver := StorageDriver(providerType)
	if !driver.IsValid() {
		return response.Error(c, fiber.StatusBadRequest, "Invalid storage provider type")
	}

	var req UpdateStorageProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// Set provider from URL param
	req.Provider = providerType

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	provider, err := h.service.UpdateStorageProvider(c.Context(), req.ID, &req)
	if err != nil {
		if err == ErrStorageProviderNotFound {
			return response.NotFound(c, "Storage provider not found")
		}
		if err == ErrConnectionFailed {
			return response.Error(c, fiber.StatusBadRequest, "Failed to connect to storage provider")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Storage provider updated successfully", ToStorageProviderResponse(provider))
}

// DeleteStorageProvider deletes a storage provider
func (h *Handler) DeleteStorageProvider(c *fiber.Ctx) error {
	providerIDStr := c.Params("provider")

	providerID, err := strconv.ParseUint(providerIDStr, 10, 32)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider ID")
	}

	if err := h.service.DeleteStorageProvider(c.Context(), uint(providerID)); err != nil {
		if err == ErrStorageProviderNotFound {
			return response.NotFound(c, "Storage provider not found")
		}
		if err == ErrStorageProviderHasBackups {
			return response.Error(c, fiber.StatusConflict, "Storage provider has associated backups and cannot be deleted")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// ShowStorageProvider shows a single storage provider
func (h *Handler) ShowStorageProvider(c *fiber.Ctx) error {
	providerIDStr := c.Params("id")

	providerID, err := strconv.ParseUint(providerIDStr, 10, 32)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid provider ID")
	}

	provider, err := h.service.GetStorageProvider(c.Context(), uint(providerID))
	if err != nil {
		if err == ErrStorageProviderNotFound {
			return response.NotFound(c, "Storage provider not found")
		}
		return response.InternalError(c, "Failed to fetch storage provider")
	}

	return response.OK(c, "Storage provider retrieved", ToStorageProviderResponse(provider))
}
