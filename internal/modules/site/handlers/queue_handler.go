package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// QueueHandler handles HTTP requests for queue workers
type QueueHandler struct {
	queueService *services.QueueService
}

// NewQueueHandler creates a new queue handler
func NewQueueHandler(queueService *services.QueueService) *QueueHandler {
	return &QueueHandler{queueService: queueService}
}

// CreateQueue creates a new queue worker
func (h *QueueHandler) CreateQueue(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateQueueRequest](c)
	if err != nil {
		return err
	}

	queue, err := h.queueService.Create(c.Context(), siteID, serverID, userID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Queue created", dto.ToQueueResponse(queue))
}

// ListQueues returns all queues for a site
func (h *QueueHandler) ListQueues(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}

	queues, err := h.queueService.List(c.Context(), siteID, serverID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	result := pkgdto.TransformSlice(queues, dto.ToQueueResponse)

	return fiberctx.OK(c, "Queues retrieved", result)
}

// UpdateQueue updates a queue worker
func (h *QueueHandler) UpdateQueue(c *fiber.Ctx) error {
	serverID, siteID, queueID, err := fiberctx.GetServerSiteAndEntityID(c, "queueId")
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateQueueRequest](c)
	if err != nil {
		return err
	}

	queue, err := h.queueService.Update(c.Context(), queueID, siteID, serverID, userID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Queue updated", dto.ToQueueResponse(queue))
}

// DeleteQueue deletes a queue
func (h *QueueHandler) DeleteQueue(c *fiber.Ctx) error {
	serverID, siteID, queueID, err := fiberctx.GetServerSiteAndEntityID(c, "queueId")
	if err != nil {
		return err
	}

	if err := h.queueService.Delete(c.Context(), queueID, siteID, serverID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Queue deletion initiated", nil)
}

// RestartQueue restarts a single queue worker
func (h *QueueHandler) RestartQueue(c *fiber.Ctx) error {
	serverID, siteID, queueID, err := fiberctx.GetServerSiteAndEntityID(c, "queueId")
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.queueService.Restart(c.Context(), queueID, siteID, serverID, userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Queue restart initiated", nil)
}

// UpdateAutoRestartQueue updates the auto-restart queue setting
func (h *QueueHandler) UpdateAutoRestartQueue(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateAutoRestartQueueRequest](c)
	if err != nil {
		return err
	}

	if err := h.queueService.UpdateAutoRestart(c.Context(), siteID, serverID, req.Enabled); err != nil {
		return fiberctx.HandleError(c, err)
	}

	message := "Auto-restart queue disabled"
	if req.Enabled {
		message = "Auto-restart queue enabled"
	}

	return fiberctx.OK(c, message, nil)
}

// SyncQueues triggers a status synchronization for all queue workers
func (h *QueueHandler) SyncQueues(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.queueService.SyncStatus(c.Context(), siteID, serverID, userID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Queue sync initiated", nil)
}
