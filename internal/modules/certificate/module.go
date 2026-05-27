// Package certificate is the team-scoped stored SSL certificate
// library. Phase 1 ships the module skeleton + table; Phase 2 adds
// the CRUD + parser + fingerprint dedupe.
//
// See docs/plans/2026-05-27-stored-certificates-design.md.
package certificate

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/certificate/handlers"
	"github.com/kkz6/launch-go/internal/modules/certificate/jobs"
	"github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/modules/certificate/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// ModuleName is the registered name for this module.
const ModuleName = "certificate"

var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
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

// RegisterJobs implements app.JobRegistrar. Binds the certificate
// module's asynq task types to their handlers.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos)
}
