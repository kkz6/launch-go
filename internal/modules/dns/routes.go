package dns

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/dns/handlers"
)

// RegisterRoutes registers all DNS module routes
func RegisterRoutes(
	router fiber.Router,
	authMiddleware fiber.Handler,
	providerHandler *handlers.DomainProviderHandler,
	domainHandler *handlers.DomainHandler,
	recordHandler *handlers.DnsRecordHandler,
) {
	// DNS Provider routes
	providers := router.Group("/dns-providers", authMiddleware, middleware.TeamScope())
	providers.Get("/", providerHandler.ListProviders)
	providers.Post("/", providerHandler.CreateProvider)
	providers.Delete("/:id", providerHandler.DeleteProvider)
	providers.Post("/:id/check", providerHandler.CheckProviderConnectivity)
	providers.Post("/:id/sync", providerHandler.SyncProviderDomains)

	// Domain routes
	domains := router.Group("/domains", authMiddleware, middleware.TeamScope())
	domains.Get("/", domainHandler.ListDomains)
	domains.Post("/", domainHandler.CreateDomain)
	domains.Get("/:id", domainHandler.ShowDomain)
	domains.Delete("/:id", domainHandler.DeleteDomain)

	// DNS Record routes
	domains.Get("/:id/records", recordHandler.ListRecords)
	domains.Post("/:id/records", recordHandler.CreateRecord)
	domains.Put("/:domainId/records/:recordId", recordHandler.UpdateRecord)
	domains.Delete("/:domainId/records/:recordId", recordHandler.DeleteRecord)

	// Utility routes
	router.Get("/dns/record-types", authMiddleware, recordHandler.GetRecordTypes)
}
