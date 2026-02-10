package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channels, err := h.service.ListChannels(c.Context(), teamID)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Notification channels retrieved", dto.ListChannelsResponse{
		Channels: dto.ToChannelResponses(channels),
	})
}

// Show returns a specific notification channel
func (h *NotificationChannelHandler) Show(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberutil.GetID(c)
	if err != nil {
		return err
	}

	channel, err := h.service.GetChannel(c.Context(), channelID, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Notification channel not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Notification channel retrieved", dto.ToChannelResponse(channel))
}

// Store creates a new notification channel
func (h *NotificationChannelHandler) Store(c *fiber.Ctx) error {
	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberutil.MustParseAndValidate[dto.CreateChannelRequest](c)
	if err != nil {
		return err
	}

	channel, err := h.service.CreateChannel(c.Context(), userID, teamID, req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidProvider) {
			return fiberutil.RespondBadRequest(c, "Invalid notification provider")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return fiberutil.RespondBadRequest(c, "Could not connect to the notification channel. Please verify your configuration.")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.Created(c, "Notification channel created", dto.ToChannelResponse(channel))
}

// Update updates an existing notification channel
func (h *NotificationChannelHandler) Update(c *fiber.Ctx) error {
	teamID, userID, err := fiberutil.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberutil.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberutil.MustParseAndValidate[dto.UpdateChannelRequest](c)
	if err != nil {
		return err
	}

	channel, err := h.service.UpdateChannel(c.Context(), channelID, userID, teamID, req)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return fiberutil.RespondBadRequest(c, "Could not connect to the notification channel. Please verify your configuration.")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Notification channel updated", dto.ToChannelResponse(channel))
}

// Destroy deletes a notification channel
func (h *NotificationChannelHandler) Destroy(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberutil.GetID(c)
	if err != nil {
		return err
	}

	err = h.service.DeleteChannel(c.Context(), channelID, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrUnauthorized) {
			return fiberutil.RespondForbidden(c, fiberutil.MsgForbidden)
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.NoContent(c)
}

// Test sends a test notification through a channel
func (h *NotificationChannelHandler) Test(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberutil.GetID(c)
	if err != nil {
		return err
	}

	// Parse optional request body - use default message if not provided or empty
	req, _ := fiberutil.MustParseAndValidate[dto.TestChannelRequest](c)
	message := "This is a test notification from Launch."
	if req != nil && req.Message != "" {
		message = req.Message
	}

	err = h.service.TestChannel(c.Context(), channelID, teamID, message)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrNotificationFailed) {
			return fiberutil.RespondBadRequest(c, "Failed to send test notification. Please verify your channel configuration.")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Test notification sent successfully", nil)
}

// SetDefault sets a channel as the default for its provider type
func (h *NotificationChannelHandler) SetDefault(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberutil.GetID(c)
	if err != nil {
		return err
	}

	err = h.service.SetChannelDefault(c.Context(), channelID, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Notification channel not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Default channel updated", nil)
}

// Disconnect marks a channel as disconnected
func (h *NotificationChannelHandler) Disconnect(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberutil.GetID(c)
	if err != nil {
		return err
	}

	err = h.service.DisconnectChannel(c.Context(), channelID, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Notification channel not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Channel disconnected", nil)
}

// Reconnect attempts to reconnect a channel
func (h *NotificationChannelHandler) Reconnect(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	channelID, err := fiberutil.GetID(c)
	if err != nil {
		return err
	}

	err = h.service.ReconnectChannel(c.Context(), channelID, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Notification channel not found")
		}

		if errors.Is(err, services.ErrConnectionFailed) {
			return fiberutil.RespondBadRequest(c, "Could not reconnect to the notification channel. Please verify your configuration.")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Channel reconnected successfully", nil)
}

// GetPreferences returns notification preferences for the current team
func (h *NotificationChannelHandler) GetPreferences(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	pref, err := h.service.GetPreferences(c.Context(), teamID)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Notification preferences retrieved", dto.ToNotificationPreferencesResponse(pref))
}

// UpdatePreferences updates notification preferences for the current team
func (h *NotificationChannelHandler) UpdatePreferences(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}

	req, err := fiberutil.MustParseAndValidate[dto.UpdateNotificationPreferencesRequest](c)
	if err != nil {
		return err
	}

	pref, err := h.service.UpdatePreferences(c.Context(), teamID, req)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Notification preferences updated", dto.ToNotificationPreferencesResponse(pref))
}

// GetService returns the underlying service for use by other modules
func (h *NotificationChannelHandler) GetService() *services.NotificationChannelService {
	return h.service
}

// ListChannelTypes returns all available notification channel types
func (h *NotificationChannelHandler) ListChannelTypes(c *fiber.Ctx) error {
	channelTypes := notificationtypes.AllChannelTypes()

	result := make([]dto.ChannelTypeResponse, len(channelTypes))
	for i, ct := range channelTypes {
		result[i] = dto.ChannelTypeResponse{
			Type:  ct.String(),
			Label: ct.Label(),
		}
	}

	return fiberutil.OK(c, "Notification channel types retrieved", result)
}
