package notification

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers the notification module routes. Standard
// CRUD on /settings/notifications and the per-channel actions wire
// directly to the framework helpers; the bespoke handlers cover Test
// (optional body), preferences PUT (singleton), and the static
// channel-type catalogue.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	svc := m.createServices()
	handler := handlers.NewNotificationChannelHandler(svc.NotificationChannel())
	chanSvc := svc.NotificationChannel()

	notifications := router.Group("/settings/notifications", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	notifications.Get("/", fiberutil.Index("Notification channels retrieved", chanSvc.ListChannels))
	notifications.Post("/", middleware.Can("notification.create"), fiberutil.Create[dto.CreateChannelRequest]("Notification channel created", chanSvc.CreateChannel))
	notifications.Get("/:id", fiberutil.Show("Notification channel retrieved", chanSvc.GetChannel))
	notifications.Put("/:id", middleware.Can("notification.update"), fiberutil.Update[dto.UpdateChannelRequest]("Notification channel updated", chanSvc.UpdateChannel))
	notifications.Delete("/:id", middleware.Can("notification.delete"), fiberutil.Delete(chanSvc.DeleteChannel))
	notifications.Post("/:id/test", middleware.Can("notification.manage"), handler.Test)
	notifications.Post("/:id/default", middleware.Can("notification.manage"), fiberutil.Action("Default channel updated", chanSvc.SetChannelDefault))
	notifications.Post("/:id/disconnect", middleware.Can("notification.manage"), fiberutil.Action("Channel disconnected", chanSvc.DisconnectChannel))
	notifications.Post("/:id/reconnect", middleware.Can("notification.manage"), fiberutil.Action("Channel reconnected successfully", chanSvc.ReconnectChannel))

	preferences := router.Group("/settings/notification-preferences", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	preferences.Get("/", fiberutil.Index("Notification preferences retrieved", chanSvc.GetPreferences))
	preferences.Put("/", middleware.Can("notification.update"), fiberutil.Validate(handler.UpdatePreferences))

	channelTypes := router.Group("/notification-channels", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	channelTypes.Get("/", handler.ListChannelTypes)
}
