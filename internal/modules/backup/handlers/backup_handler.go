package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// BackupHandler handles HTTP requests for backups
type BackupHandler struct {
	backupService *services.BackupService
}

// NewBackupHandler creates a new backup handler
func NewBackupHandler(backupService *services.BackupService) *BackupHandler {
	return &BackupHandler{backupService: backupService}
}

// ListBackups lists all backups for a server
func (h *BackupHandler) ListBackups(c *fiber.Ctx) error {
	serverID := c.Params("serverId")

	backups, err := h.backupService.ListBackupsByServer(c.Context(), serverID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch backups")
	}

	result := make([]dto.BackupResponse, len(backups))
	for i, backup := range backups {
		result[i] = dto.ToBackupResponse(&backup)
	}

	return response.OK(c, "Backups retrieved", result)
}

// CreateBackup creates a new backup configuration
func (h *BackupHandler) CreateBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateBackupRequest](c)
	if err != nil {
		return err
	}

	backup, err := h.backupService.CreateBackup(c.Context(), serverID, userID, teamID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Backup created successfully", dto.ToBackupResponse(backup))
}

// ShowBackup shows a single backup
func (h *BackupHandler) ShowBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	backup, err := h.backupService.GetBackupByIDAndServer(c.Context(), backupID, serverID)
	if err != nil {
		if err == repositories.ErrBackupNotFound {
			return response.NotFound(c, response.MsgBackupNotFound)
		}
		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Backup retrieved", dto.ToBackupResponse(backup))
}

// UpdateBackup updates a backup configuration
func (h *BackupHandler) UpdateBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	// Verify backup belongs to server
	_, err := h.backupService.GetBackupByIDAndServer(c.Context(), backupID, serverID)
	if err != nil {
		if err == repositories.ErrBackupNotFound {
			return response.NotFound(c, response.MsgBackupNotFound)
		}
		return response.InternalError(c, response.MsgInternalError)
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateBackupRequest](c)
	if err != nil {
		return err
	}

	backup, err := h.backupService.UpdateBackup(c.Context(), backupID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Backup updated successfully", dto.ToBackupResponse(backup))
}

// DeleteBackup deletes a backup configuration
func (h *BackupHandler) DeleteBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	if err := h.backupService.DeleteBackup(c.Context(), backupID, serverID); err != nil {
		if err == repositories.ErrBackupNotFound {
			return response.NotFound(c, response.MsgBackupNotFound)
		}
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// RunManualBackup triggers a manual backup run
func (h *BackupHandler) RunManualBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	if err := h.backupService.RunBackup(c.Context(), backupID, serverID); err != nil {
		if err == repositories.ErrBackupNotFound {
			return response.NotFound(c, response.MsgBackupNotFound)
		}
		return response.HandleError(c, err)
	}

	return response.OK(c, "Backup queued for execution", nil)
}
