package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
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

	var req dto.CreateBackupJobRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	_, err := h.jobService.CreateBackupJob(c.Context(), backupID, token, &req)
	if err != nil {
		if err == services.ErrInvalidDispatchToken {
			return response.Forbidden(c, "Invalid dispatch token")
		}
		if err == repositories.ErrBackupNotFound {
			return response.NotFound(c, "Backup not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return c.SendStatus(fiber.StatusNoContent)
}
