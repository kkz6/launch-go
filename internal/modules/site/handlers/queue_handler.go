package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// QueueHandler holds the queue endpoint that does not fit a generic
// route helper: UpdateAutoRestartQueue is a PUT to a site-singleton
// setting (no own id) with a body and a dynamic success message.
type QueueHandler struct {
	queueService *services.QueueService
}

// NewQueueHandler creates a new queue handler.
func NewQueueHandler(queueService *services.QueueService) *QueueHandler {
	return &QueueHandler{queueService: queueService}
}

// UpdateAutoRestartQueue toggles the auto-restart queue setting on a site.
func (h *QueueHandler) UpdateAutoRestartQueue(c *fiber.Ctx, req *dto.UpdateAutoRestartQueueRequest) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	if err := h.queueService.UpdateAutoRestart(c.Context(), siteID, serverID, teamID, req.Enabled); err != nil {
		return err
	}
	message := "Auto-restart queue disabled"
	if req.Enabled {
		message = "Auto-restart queue enabled"
	}
	return fiberctx.OK(c, message, nil)
}
