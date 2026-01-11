package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// NotificationChannelHandler handles HTTP requests for notification channels
type NotificationChannelHandler struct {
	service *services.NotificationChannelService
}

// NewNotificationChannelHandler creates a new notification channel handler
func NewNotificationChannelHandler(service *services.NotificationChannelService) *NotificationChannelHandler {
	return &NotificationChannelHandler{service: service}
}

// Index returns all notification channels for the current team
func (h *NotificationChannelHandler) Index(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	channels, err := h.service.ListChannels(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to retrieve notification channels")
	}

	return response.OK(c, "Notification channels retrieved", dto.ListChannelsResponse{
		Channels: dto.ToChannelResponses(channels),
	})
}

// Show returns a specific notification channel
func (h *NotificationChannelHandler) Show(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	channel, err := h.service.GetChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		return response.InternalError(c, "Failed to retrieve notification channel")
	}

	return response.OK(c, "Notification channel retrieved", dto.ToChannelResponse(channel))
}

// Store creates a new notification channel
func (h *NotificationChannelHandler) Store(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)

	var req dto.CreateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	channel, err := h.service.CreateChannel(c.Context(), userID, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidProvider) {
			return response.Error(c, fiber.StatusBadRequest, "Invalid notification provider")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return response.Error(c, fiber.StatusBadRequest, "Could not connect to the notification channel. Please verify your configuration.")
		}

		return response.InternalError(c, "Failed to create notification channel")
	}

	return response.Created(c, "Notification channel created", dto.ToChannelResponse(channel))
}

// Update updates an existing notification channel
func (h *NotificationChannelHandler) Update(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	var req dto.UpdateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	channel, err := h.service.UpdateChannel(c.Context(), channelID, userID, teamID, &req)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return response.Error(c, fiber.StatusBadRequest, "Could not connect to the notification channel. Please verify your configuration.")
		}

		return response.InternalError(c, "Failed to update notification channel")
	}

	return response.OK(c, "Notification channel updated", dto.ToChannelResponse(channel))
}

// Destroy deletes a notification channel
func (h *NotificationChannelHandler) Destroy(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	err := h.service.DeleteChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrUnauthorized) {
			return response.Forbidden(c, "You do not have permission to delete this channel")
		}

		return response.InternalError(c, "Failed to delete notification channel")
	}

	return response.NoContent(c)
}

// Test sends a test notification through a channel
func (h *NotificationChannelHandler) Test(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	var req dto.TestChannelRequest
	if err := c.BodyParser(&req); err != nil {
		// If body parsing fails, use default message
		req.Message = "This is a test notification from Launch."
	}

	if req.Message == "" {
		req.Message = "This is a test notification from Launch."
	}

	err := h.service.TestChannel(c.Context(), channelID, teamID, req.Message)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrNotificationFailed) {
			return response.Error(c, fiber.StatusBadRequest, "Failed to send test notification. Please verify your channel configuration.")
		}

		return response.InternalError(c, "Failed to send test notification")
	}

	return response.OK(c, "Test notification sent successfully", nil)
}

// SetDefault sets a channel as the default for its provider type
func (h *NotificationChannelHandler) SetDefault(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	err := h.service.SetChannelDefault(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		return response.InternalError(c, "Failed to set default channel")
	}

	return response.OK(c, "Default channel updated", nil)
}

// Disconnect marks a channel as disconnected
func (h *NotificationChannelHandler) Disconnect(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	err := h.service.DisconnectChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		return response.InternalError(c, "Failed to disconnect channel")
	}

	return response.OK(c, "Channel disconnected", nil)
}

// Reconnect attempts to reconnect a channel
func (h *NotificationChannelHandler) Reconnect(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	err := h.service.ReconnectChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return response.Error(c, fiber.StatusBadRequest, "Could not reconnect to the notification channel. Please verify your configuration.")
		}

		return response.InternalError(c, "Failed to reconnect channel")
	}

	return response.OK(c, "Channel reconnected successfully", nil)
}

// GetService returns the underlying service for use by other modules
func (h *NotificationChannelHandler) GetService() *services.NotificationChannelService {
	return h.service
}
