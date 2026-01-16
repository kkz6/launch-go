package dns

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/dns/handlers"
)

// RegisterRoutes registers all DNS module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Create services
	svc := m.createServices()

	// Create handlers
	providerHandler := handlers.NewDomainProviderHandler(svc.Provider())
	domainHandler := handlers.NewDomainHandler(svc.Domain(), svc.Provider())
	recordHandler := handlers.NewDnsRecordHandler(svc.Record(), svc.Domain())

	m.registerProviderRoutes(router, authMiddleware, providerHandler)
	m.registerDomainRoutes(router, authMiddleware, domainHandler, recordHandler)
	m.registerUtilityRoutes(router, authMiddleware, recordHandler)
}

// registerProviderRoutes registers DNS provider routes
func (m *Module) registerProviderRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.DomainProviderHandler) {
	providers := router.Group("/dns-providers", authMiddleware, middleware.TeamScope())
	{
		providers.Get("/", handler.ListProviders)
		providers.Post("/", handler.CreateProvider)
		providers.Delete("/:id", handler.DeleteProvider)
		providers.Post("/:id/check", handler.CheckProviderConnectivity)
		providers.Post("/:id/sync", handler.SyncProviderDomains)
	}
}

// registerDomainRoutes registers domain and DNS record routes
func (m *Module) registerDomainRoutes(router fiber.Router, authMiddleware fiber.Handler, domainHandler *handlers.DomainHandler, recordHandler *handlers.DnsRecordHandler) {
	domains := router.Group("/dns/domains", authMiddleware, middleware.TeamScope())
	{
		domains.Get("/", domainHandler.ListDomains)
		domains.Post("/", domainHandler.CreateDomain)
		domains.Get("/:id", domainHandler.ShowDomain)
		domains.Patch("/:id", domainHandler.UpdateDomain)
		domains.Delete("/:id", domainHandler.DeleteDomain)
		domains.Post("/:id/sync", domainHandler.SyncDomain)

		// DNS Record routes
		domains.Get("/:id/records", recordHandler.ListRecords)
		domains.Post("/:id/records", recordHandler.CreateRecord)
		domains.Post("/:domainId/records/:recordId", recordHandler.UpdateRecord)
		domains.Delete("/:domainId/records/:recordId", recordHandler.DeleteRecord)
	}
}

// registerUtilityRoutes registers utility routes
func (m *Module) registerUtilityRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.DnsRecordHandler) {
	router.Get("/dns/record-types", authMiddleware, handler.GetRecordTypes)
}
