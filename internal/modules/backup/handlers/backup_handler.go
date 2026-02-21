package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
		return fiberutil.HandleError(c, err)
	}

	result := pkgdto.TransformSlice(backups, dto.ToBackupResponse)

	return fiberutil.OK(c, "Backups retrieved", result)
}

// CreateBackup creates a new backup configuration
func (h *BackupHandler) CreateBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberutil.MustParseAndValidate[dto.CreateBackupRequest](c)
	if err != nil {
		return err
	}

	backup, err := h.backupService.CreateBackup(c.Context(), serverID, userID, teamID, req)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.Created(c, "Backup created successfully", dto.ToBackupResponse(backup))
}

// ShowBackup shows a single backup
func (h *BackupHandler) ShowBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	backup, err := h.backupService.GetBackupByIDAndServer(c.Context(), backupID, serverID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Backup not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Backup retrieved", dto.ToBackupResponse(backup))
}

// UpdateBackup updates a backup configuration
func (h *BackupHandler) UpdateBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	// Verify backup belongs to server
	_, err := h.backupService.GetBackupByIDAndServer(c.Context(), backupID, serverID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Backup not found")
		}
		return fiberutil.HandleError(c, err)
	}

	req, err := fiberutil.MustParseAndValidate[dto.UpdateBackupRequest](c)
	if err != nil {
		return err
	}

	backup, err := h.backupService.UpdateBackup(c.Context(), backupID, req)
	if err != nil {
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Backup updated successfully", dto.ToBackupResponse(backup))
}

// DeleteBackup deletes a backup configuration
func (h *BackupHandler) DeleteBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	if err := h.backupService.DeleteBackup(c.Context(), backupID, serverID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Backup not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.NoContent(c)
}

// RunManualBackup triggers a manual backup run
func (h *BackupHandler) RunManualBackup(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	backupID := c.Params("id")

	if err := h.backupService.RunBackup(c.Context(), backupID, serverID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Backup not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Backup queued for execution", nil)
}
