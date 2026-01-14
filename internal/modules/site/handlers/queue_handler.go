package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
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
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	var req dto.CreateQueueRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	queue, err := h.queueService.Create(c.Context(), siteID, serverID, userID, &req)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Queue created", dto.ToQueueResponse(queue))
}

// ListQueues returns all queues for a site
func (h *QueueHandler) ListQueues(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	queues, err := h.queueService.List(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.InternalError(c, "Failed to fetch queues")
	}

	result := make([]dto.QueueResponse, len(queues))
	for i, queue := range queues {
		result[i] = dto.ToQueueResponse(&queue)
	}

	return response.OK(c, "Queues retrieved", result)
}

// DeleteQueue deletes a queue
func (h *QueueHandler) DeleteQueue(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	queueID := c.Params("queueId")

	if err := h.queueService.Delete(c.Context(), queueID, siteID, serverID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		if errors.Is(err, repositories.ErrQueueNotFound) {
			return response.NotFound(c, "Queue not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Queue deletion initiated", nil)
}

// EnableAutoRestartQueue enables auto-restart for queue workers
func (h *QueueHandler) EnableAutoRestartQueue(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.queueService.EnableAutoRestart(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Auto-restart queue enabled", nil)
}

// DisableAutoRestartQueue disables auto-restart for queue workers
func (h *QueueHandler) DisableAutoRestartQueue(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	if err := h.queueService.DisableAutoRestart(c.Context(), siteID, serverID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Auto-restart queue disabled", nil)
}

// SyncQueues triggers a status synchronization for all queue workers
func (h *QueueHandler) SyncQueues(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID := c.Locals("userID").(string)

	if err := h.queueService.SyncStatus(c.Context(), siteID, serverID, userID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, "Site not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Queue sync initiated", nil)
}
