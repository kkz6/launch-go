// Package certificate is the team-scoped stored SSL certificate
// library. Phase 1 ships the module skeleton + table; Phase 2 adds
// the CRUD + parser + fingerprint dedupe.
//
// See docs/plans/2026-05-27-stored-certificates-design.md.
package certificate

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/certificate/handlers"
	"github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/modules/certificate/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// ModuleName is the registered name for this module.
const ModuleName = "certificate"

var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module wires the certificate module into the framework.
type Module struct {
	app.Base
	repos   *repositories.Registry
	service *services.StoredCertificateService
	handler *handlers.StoredCertificateHandler
}

// NewModule constructs the module.
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)
	svc := services.NewStoredCertificateService(repos)
	return &Module{
		Base:    app.NewBase(ModuleName, b),
		repos:   repos,
		service: svc,
		handler: handlers.NewStoredCertificateHandler(svc),
	}
}

// Repos exposes the repository registry for cross-module access.
func (m *Module) Repos() *repositories.Registry { return m.repos }

// RegisterRoutes mounts the certificate module's HTTP routes under
// /api/certificates. The AuthenticatedChain wraps the group with
// auth + team-scope + subscription middleware.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	auth := middleware.AuthenticatedChain(authMiddleware)

	grp := router.Group("/certificates", auth...)
	grp.Get("/", m.handler.List)
	grp.Post("/", m.handler.Create)
	grp.Get("/:id", m.handler.Get)
	grp.Patch("/:id", m.handler.Update)
	grp.Delete("/:id", m.handler.Delete)
	grp.Get("/:id/usages", m.handler.Usages)
}
