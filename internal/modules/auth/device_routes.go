package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func (m *Module) registerDeviceRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	deps := m.Deps()
	handler := handlers.NewDeviceHandler(deps.DB, m.service.TwoFactor, deps.Config.JWT.Secret, deps.Config.App.Frontend())
	device := router.Group("/device", scrubDeviceRequest)
	device.Post("/code", handler.Code)
	device.Post("/token", fiberutil.Validate(handler.Token))
	device.Post("/authorize", authMiddleware, fiberutil.Validate(handler.Authorize))
}
