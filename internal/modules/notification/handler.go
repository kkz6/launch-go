package notification

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for notifications
type Handler struct {
	service *Service
}

// NewHandler creates a new notification handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Index returns all notification channels for the current team
func (h *Handler) Index(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	channels, err := h.service.ListChannels(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to retrieve notification channels")
	}

	return response.OK(c, "Notification channels retrieved", ListChannelsResponse{
		Channels: ToChannelResponses(channels),
	})
}

// Show returns a specific notification channel
func (h *Handler) Show(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	channel, err := h.service.GetChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}
		return response.InternalError(c, "Failed to retrieve notification channel")
	}

	return response.OK(c, "Notification channel retrieved", ToChannelResponse(channel))
}

// Store creates a new notification channel
func (h *Handler) Store(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)

	var req CreateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	channel, err := h.service.CreateChannel(c.Context(), userID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrInvalidProvider) {
			return response.Error(c, fiber.StatusBadRequest, "Invalid notification provider")
		}
		if errors.Is(err, ErrConnectionFailed) {
			return response.Error(c, fiber.StatusBadRequest, "Could not connect to the notification channel. Please verify your configuration.")
		}
		return response.InternalError(c, "Failed to create notification channel")
	}

	return response.Created(c, "Notification channel created", ToChannelResponse(channel))
}

// Update updates an existing notification channel
func (h *Handler) Update(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	_ = userID // userID reserved for future authorization checks

	var req UpdateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	channel, err := h.service.UpdateChannel(c.Context(), channelID, userID, teamID, &req)
	if err != nil {
		if errors.Is(err, ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}
		if errors.Is(err, ErrConnectionFailed) {
			return response.Error(c, fiber.StatusBadRequest, "Could not connect to the notification channel. Please verify your configuration.")
		}
		return response.InternalError(c, "Failed to update notification channel")
	}

	return response.OK(c, "Notification channel updated", ToChannelResponse(channel))
}

// Destroy deletes a notification channel
func (h *Handler) Destroy(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	err := h.service.DeleteChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}
		if errors.Is(err, ErrUnauthorized) {
			return response.Forbidden(c, "You do not have permission to delete this channel")
		}
		return response.InternalError(c, "Failed to delete notification channel")
	}

	return response.NoContent(c)
}

// Test sends a test notification through a channel
func (h *Handler) Test(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	var req TestChannelRequest
	if err := c.BodyParser(&req); err != nil {
		// If body parsing fails, use default message
		req.Message = "This is a test notification from Launch."
	}

	if req.Message == "" {
		req.Message = "This is a test notification from Launch."
	}

	err := h.service.TestChannel(c.Context(), channelID, teamID, req.Message)
	if err != nil {
		if errors.Is(err, ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}
		if errors.Is(err, ErrNotificationFailed) {
			return response.Error(c, fiber.StatusBadRequest, "Failed to send test notification. Please verify your channel configuration.")
		}
		return response.InternalError(c, "Failed to send test notification")
	}

	return response.OK(c, "Test notification sent successfully", nil)
}

// SetDefault sets a channel as the default for its provider type
func (h *Handler) SetDefault(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	err := h.service.SetChannelDefault(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}
		return response.InternalError(c, "Failed to set default channel")
	}

	return response.OK(c, "Default channel updated", nil)
}

// Disconnect marks a channel as disconnected
func (h *Handler) Disconnect(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	err := h.service.DisconnectChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}
		return response.InternalError(c, "Failed to disconnect channel")
	}

	return response.OK(c, "Channel disconnected", nil)
}

// Reconnect attempts to reconnect a channel
func (h *Handler) Reconnect(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	channelID := c.Params("id")

	err := h.service.ReconnectChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}
		if errors.Is(err, ErrConnectionFailed) {
			return response.Error(c, fiber.StatusBadRequest, "Could not reconnect to the notification channel. Please verify your configuration.")
		}
		return response.InternalError(c, "Failed to reconnect channel")
	}

	return response.OK(c, "Channel reconnected successfully", nil)
}
