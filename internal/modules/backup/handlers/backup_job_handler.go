package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// BackupJobHandler handles HTTP requests for backup jobs
type BackupJobHandler struct {
	jobService *services.BackupJobService
}

// NewBackupJobHandler creates a new backup job handler
func NewBackupJobHandler(jobService *services.BackupJobService) *BackupJobHandler {
	return &BackupJobHandler{jobService: jobService}
}

// CreateBackupJob creates a new backup job (webhook endpoint for agent)
func (h *BackupJobHandler) CreateBackupJob(c *fiber.Ctx) error {
	backupID := c.Params("backup")
	token := c.Params("token")

	req, err := fiberutil.MustParseAndValidate[dto.CreateBackupJobRequest](c)
	if err != nil {
		return err
	}

	_, err = h.jobService.CreateBackupJob(c.Context(), backupID, token, req)
	if err != nil {
		if err == services.ErrInvalidDispatchToken {
			return response.Forbidden(c, response.MsgForbidden)
		}
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgBackupNotFound)
		}
		return response.HandleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
