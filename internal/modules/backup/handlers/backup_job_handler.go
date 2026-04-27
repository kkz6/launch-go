package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// BackupJobHandler exposes the public agent webhook for backup job
// status reports. The webhook is unauthenticated (dispatch-token guarded)
// so it does not fit the team-scoped route helpers.
type BackupJobHandler struct {
	jobService *services.BackupJobService
}

// NewBackupJobHandler creates a new backup job handler.
func NewBackupJobHandler(jobService *services.BackupJobService) *BackupJobHandler {
	return &BackupJobHandler{jobService: jobService}
}

// CreateBackupJob is the agent webhook for reporting a backup run result.
func (h *BackupJobHandler) CreateBackupJob(c *fiber.Ctx) error {
	req, err := fiberutil.MustParseAndValidate[dto.CreateBackupJobRequest](c)
	if err != nil {
		return err
	}

	if _, err := h.jobService.CreateBackupJob(c.Context(), c.Params("backup"), c.Params("token"), req); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
