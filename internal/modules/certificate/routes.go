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
	grp.Get("/", m.handler.List)
	grp.Post("/", middleware.Can("certificate.create"), fiberutil.Validate(m.handler.Create))
	grp.Get("/:id", m.handler.Get)
	grp.Patch("/:id", middleware.Can("certificate.update"), fiberutil.Validate(m.handler.Update))
	grp.Delete("/:id", middleware.Can("certificate.delete"), m.handler.Delete)
	grp.Get("/:id/usages", m.handler.Usages)
}
