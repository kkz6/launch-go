package dns

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/middleware"
)

// Module represents the DNS module
type Module struct {
	handler *Handler
	service *Service
	repo    *Repository
}

// NewModule creates a new DNS module instance
func NewModule(db *gorm.DB, logger *zerolog.Logger) *Module {
	repo := NewRepository(db)
	service := NewService(repo, logger)
	handler := NewHandler(service)

	return &Module{
		handler: handler,
		service: service,
		repo:    repo,
	}
}

// RegisterRoutes registers all DNS module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// DNS Provider routes
	providers := router.Group("/dns-providers", authMiddleware, middleware.TeamScope())
	providers.Get("/", m.handler.ListProviders)
	providers.Post("/", m.handler.CreateProvider)
	providers.Delete("/:id", m.handler.DeleteProvider)
	providers.Post("/:id/check", m.handler.CheckProviderConnectivity)
	providers.Post("/:id/sync", m.handler.SyncProviderDomains)

	// Domain routes
	domains := router.Group("/domains", authMiddleware, middleware.TeamScope())
	domains.Get("/", m.handler.ListDomains)
	domains.Post("/", m.handler.CreateDomain)
	domains.Get("/:id", m.handler.ShowDomain)
	domains.Delete("/:id", m.handler.DeleteDomain)

	// DNS Record routes
	domains.Get("/:id/records", m.handler.ListRecords)
	domains.Post("/:id/records", m.handler.CreateRecord)
	domains.Put("/:domainId/records/:recordId", m.handler.UpdateRecord)
	domains.Delete("/:domainId/records/:recordId", m.handler.DeleteRecord)

	// Utility routes
	router.Get("/dns/record-types", authMiddleware, m.handler.GetRecordTypes)
}

// GetService returns the DNS service
func (m *Module) GetService() *Service {
	return m.service
}

// GetRepository returns the DNS repository
func (m *Module) GetRepository() *Repository {
	return m.repo
}

// AutoMigrate runs database migrations for the DNS module
func (m *Module) AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&DomainProvider{},
		&Domain{},
		&DnsRecord{},
	)
}
