package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// NotificationChannelHandler holds the notification endpoints that do
// not fit the team-scoped route helpers. Standard CRUD and the
// disconnect / reconnect / set-default actions are wired directly to the
// framework helpers in routes.go.
type NotificationChannelHandler struct {
	service *services.NotificationChannelService
}

// NewNotificationChannelHandler creates a new notification channel handler.
func NewNotificationChannelHandler(service *services.NotificationChannelService) *NotificationChannelHandler {
	return &NotificationChannelHandler{service: service}
}

// Test sends a test notification through a channel. The request body is
// optional; an empty or invalid body falls back to the default message.
func (h *NotificationChannelHandler) Test(r *fiberutil.Request) error {
	channelID, err := fiberutil.GetID(r.Ctx)
	if err != nil {
		return err
	}

	message := "This is a test notification from Launch."
	if req, perr := fiberutil.MustParseAndValidate[dto.TestChannelRequest](r.Ctx); perr == nil && req.Message != "" {
		message = req.Message
	}

	if err := h.service.TestChannel(r.Context(), channelID, r.TeamID, message); err != nil {
		return err
	}
	return fiberutil.OK(r.Ctx, "Test notification sent successfully", nil)
}

// UpdatePreferences updates notification preferences for the current
// team. PUT to a team-singleton resource — no :id, body required — so
// it does not fit the generic Update helper.
func (h *NotificationChannelHandler) UpdatePreferences(r *fiberutil.Request, req *dto.UpdateNotificationPreferencesRequest) error {
	resp, err := h.service.UpdatePreferences(r.Context(), r.TeamID, req)
	if err != nil {
		return err
	}
	return fiberutil.OK(r.Ctx, "Notification preferences updated", resp)
}

// ListChannelTypes returns all available notification channel types.
// Static catalogue, not team-scoped — no MustGet to remove.
func (h *NotificationChannelHandler) ListChannelTypes(c *fiber.Ctx) error {
	channelTypes := notificationtypes.AllChannelTypes()
	result := make([]dto.ChannelTypeResponse, len(channelTypes))
	for i, ct := range channelTypes {
		result[i] = dto.ChannelTypeResponse{Type: ct.String(), Label: ct.Label()}
	}
	return fiberutil.OK(c, "Notification channel types retrieved", result)
}

// GetService returns the underlying service for use by other modules.
func (h *NotificationChannelHandler) GetService() *services.NotificationChannelService {
	return h.service
}
