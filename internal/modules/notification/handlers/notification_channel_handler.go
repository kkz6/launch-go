package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
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
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channels, err := h.service.ListChannels(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Notification channels retrieved", dto.ListChannelsResponse{
		Channels: dto.ToChannelResponses(channels),
	})
}

// Show returns a specific notification channel
func (h *NotificationChannelHandler) Show(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	channel, err := h.service.GetChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Notification channel retrieved", dto.ToChannelResponse(channel))
}

// Store creates a new notification channel
func (h *NotificationChannelHandler) Store(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateChannelRequest](c)
	if err != nil {
		return err
	}

	channel, err := h.service.CreateChannel(c.Context(), userID, teamID, req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidProvider) {
			return response.BadRequest(c, "Invalid notification provider")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return response.BadRequest(c, "Could not connect to the notification channel. Please verify your configuration.")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.Created(c, "Notification channel created", dto.ToChannelResponse(channel))
}

// Update updates an existing notification channel
func (h *NotificationChannelHandler) Update(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateChannelRequest](c)
	if err != nil {
		return err
	}

	channel, err := h.service.UpdateChannel(c.Context(), channelID, userID, teamID, req)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return response.BadRequest(c, "Could not connect to the notification channel. Please verify your configuration.")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Notification channel updated", dto.ToChannelResponse(channel))
}

// Destroy deletes a notification channel
func (h *NotificationChannelHandler) Destroy(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	err = h.service.DeleteChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrUnauthorized) {
			return response.Forbidden(c, response.MsgForbidden)
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.NoContent(c)
}

// Test sends a test notification through a channel
func (h *NotificationChannelHandler) Test(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	// Parse optional request body - use default message if not provided or empty
	req, _ := fiberctx.MustParseAndValidate[dto.TestChannelRequest](c)
	message := "This is a test notification from Launch."
	if req != nil && req.Message != "" {
		message = req.Message
	}

	err = h.service.TestChannel(c.Context(), channelID, teamID, message)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrNotificationFailed) {
			return response.BadRequest(c, "Failed to send test notification. Please verify your channel configuration.")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Test notification sent successfully", nil)
}

// SetDefault sets a channel as the default for its provider type
func (h *NotificationChannelHandler) SetDefault(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	err = h.service.SetChannelDefault(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Default channel updated", nil)
}

// Disconnect marks a channel as disconnected
func (h *NotificationChannelHandler) Disconnect(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	err = h.service.DisconnectChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Channel disconnected", nil)
}

// Reconnect attempts to reconnect a channel
func (h *NotificationChannelHandler) Reconnect(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	err = h.service.ReconnectChannel(c.Context(), channelID, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrChannelNotFound) {
			return response.NotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return response.BadRequest(c, "Could not reconnect to the notification channel. Please verify your configuration.")
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	return response.OK(c, "Channel reconnected successfully", nil)
}

// GetService returns the underlying service for use by other modules
func (h *NotificationChannelHandler) GetService() *services.NotificationChannelService {
	return h.service
}

// ListChannelTypes returns all available notification channel types
func (h *NotificationChannelHandler) ListChannelTypes(c *fiber.Ctx) error {
	channelTypes := enums.AllChannelTypes()

	result := make([]dto.ChannelTypeResponse, len(channelTypes))
	for i, ct := range channelTypes {
		result[i] = dto.ChannelTypeResponse{
			Type:  ct.String(),
			Label: ct.Label(),
		}
	}

	return response.OK(c, "Notification channel types retrieved", result)
}
