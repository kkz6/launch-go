package dns

import (
	gofiber "github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/handlers"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers all DNS module routes. Most endpoints fit one
// of the team-scoped route helpers in pkg/fiber, so the wiring here is
// effectively a route table.
func (m *Module) RegisterRoutes(router gofiber.Router, authMiddleware gofiber.Handler) {
	svc := m.createServices()

	domainHandler := handlers.NewDomainHandler(svc.Domain())
	recordHandler := handlers.NewDNSRecordHandler(svc.Record())

	// Static record-types lookup (not team-scoped).
	router.Get("/dns/record-types", authMiddleware, recordHandler.GetRecordTypes)

	m.registerProviderRoutes(router, authMiddleware, svc)
	m.registerDomainRoutes(router, authMiddleware, svc, domainHandler)
}

func (m *Module) registerProviderRoutes(router gofiber.Router, authMiddleware gofiber.Handler, svc *services.ServiceRegistry) {
	g := router.Group("/dns-providers", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	g.Get("/", fiberutil.Index("Providers retrieved", svc.Provider().ListProviders))
	g.Post("/", fiberutil.Create[dto.CreateDomainProviderRequest]("Provider created", svc.Provider().CreateProvider))
	g.Delete("/:id", fiberutil.Delete(svc.Provider().DeleteProvider))
	g.Post("/:id/check", fiberutil.Action("Provider is connected", svc.Provider().CheckProviderConnectivity))
	g.Post("/:id/sync", fiberutil.Action("Domains synchronized successfully", svc.Provider().SyncDomains))
}

func (m *Module) registerDomainRoutes(router gofiber.Router, authMiddleware gofiber.Handler, svc *services.ServiceRegistry, domainHandler *handlers.DomainHandler) {
	g := router.Group("/dns/domains", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	g.Get("/", fiberutil.Index("Domains retrieved", svc.Domain().ListDomainsPage))
	g.Post("/", fiberutil.Create[dto.CreateDomainRequest]("Domain created", svc.Domain().CreateDomain))
	g.Get("/:id", fiberutil.Show("Domain retrieved", svc.Domain().GetDomainPage))
	g.Patch("/:id", fiberutil.Update[dto.UpdateDomainRequest]("Domain updated", svc.Domain().UpdateDomain))
	g.Delete("/:id", domainHandler.DeleteDomain)
	g.Post("/:id/sync", fiberutil.Action("DNS records synced successfully", svc.Domain().SyncDomainRecords))

	// Nested DNS records.
	g.Get("/:id/records", fiberutil.IndexNested("id", "Records retrieved", svc.Domain().GetDomainRecords))
	g.Post("/:id/records", fiberutil.CreateNested[dto.CreateDNSRecordRequest]("id", "Record created", svc.Record().CreateRecord))
	g.Post("/:domainId/records/:recordId", fiberutil.UpdateNested[dto.UpdateDNSRecordRequest]("domainId", "recordId", "Record updated", svc.Record().UpdateRecord))
	g.Delete("/:domainId/records/:recordId", fiberutil.DeleteNested("domainId", "recordId", svc.Record().DeleteRecord))
}
