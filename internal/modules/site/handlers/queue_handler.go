package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
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
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
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
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.Created(c, "Queue created", dto.ToQueueResponse(queue))
}

// ListQueues returns all queues for a site
func (h *QueueHandler) ListQueues(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	queues, err := h.queueService.List(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	result := make([]dto.QueueResponse, len(queues))
	for i, queue := range queues {
		result[i] = dto.ToQueueResponse(&queue)
	}

	return response.OK(c, "Queues retrieved", result)
}

// DeleteQueue deletes a queue
func (h *QueueHandler) DeleteQueue(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	queueID, err := fiberctx.GetULIDParam(c, "queueId")
	if err != nil {
		return err
	}

	if err := h.queueService.Delete(c.Context(), queueID, siteID, serverID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		if errors.Is(err, repositories.ErrQueueNotFound) {
			return response.NotFound(c, response.MsgQueueNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Queue deletion initiated", nil)
}

// UpdateAutoRestartQueue updates the auto-restart queue setting
func (h *QueueHandler) UpdateAutoRestartQueue(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateAutoRestartQueueRequest](c)
	if err != nil {
		return err
	}

	if err := h.queueService.UpdateAutoRestart(c.Context(), siteID, serverID, req.Enabled); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	message := "Auto-restart queue disabled"
	if req.Enabled {
		message = "Auto-restart queue enabled"
	}

	return response.OK(c, message, nil)
}

// SyncQueues triggers a status synchronization for all queue workers
func (h *QueueHandler) SyncQueues(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	if err := h.queueService.SyncStatus(c.Context(), siteID, serverID, userID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Queue sync initiated", nil)
}
