package dockerregistry

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/dto"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes wires the docker-registry HTTP routes. Team-scoped, no
// server scope — credentials live at the team level and are picked up
// at deploy time on whichever server the application targets.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	svc := m.service

	g := router.Group(
		"/docker-registries",
		authMiddleware,
		middleware.TeamScope(),
		middleware.VerifySubscription(),
	)

	g.Get("/", fiberutil.Index("Docker registry credentials retrieved", svc.List))
	g.Post("/", fiberutil.Create[dto.CreateCredentialRequest]("Docker registry credential created", svc.Create))
	g.Get("/:id", fiberutil.Show("Docker registry credential retrieved", svc.Get))
	g.Put("/:id", fiberutil.Update[dto.UpdateCredentialRequest]("Docker registry credential updated", svc.Update))
	g.Delete("/:id", fiberutil.Delete(svc.Delete))
}
