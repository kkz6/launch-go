package certificate

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes mounts the certificate module's HTTP routes under
// /certificates. The AuthenticatedChain wraps the group with auth +
// team-scope + subscription middleware.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	auth := middleware.AuthenticatedChain(authMiddleware)

	grp := router.Group("/certificates", auth...)
	grp.Get("/", fiberutil.Handler(m.handler.List))
	grp.Post("/", middleware.Can("certificate.create"), fiberutil.Bind(m.handler.Create))
	grp.Get("/:id", fiberutil.Handler(m.handler.Get))
	grp.Patch("/:id", middleware.Can("certificate.update"), fiberutil.Bind(m.handler.Update))
	grp.Delete("/:id", middleware.Can("certificate.delete"), fiberutil.Handler(m.handler.Delete))
	grp.Get("/:id/usages", fiberutil.Handler(m.handler.Usages))
}
