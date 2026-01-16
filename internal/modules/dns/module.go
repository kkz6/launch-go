package dns

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/handlers"
	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
)

const ModuleName = "dns"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module represents the DNS module
type Module struct {
	module.Base
	providerHandler *handlers.DomainProviderHandler
	domainHandler   *handlers.DomainHandler
	recordHandler   *handlers.DnsRecordHandler

	providerService *services.DomainProviderService
	domainService   *services.DomainService
	recordService   *services.DnsRecordService

	providerRepo  *repositories.DomainProviderRepository
	domainRepo    *repositories.DomainRepository
	dnsRecordRepo *repositories.DnsRecordRepository
}

// NewModule creates a new DNS module instance
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	// Initialize repositories
	providerRepo := repositories.NewDomainProviderRepository(deps.DB)
	domainRepo := repositories.NewDomainRepository(deps.DB)
	dnsRecordRepo := repositories.NewDnsRecordRepository(deps.DB)

	// Initialize services
	providerService := services.NewDomainProviderService(providerRepo, domainRepo, dnsRecordRepo, deps.Logger)
	domainService := services.NewDomainService(providerRepo, domainRepo, dnsRecordRepo, deps.Logger)
	recordService := services.NewDnsRecordService(domainRepo, dnsRecordRepo, deps.Logger)

	// Initialize handlers
	providerHandler := handlers.NewDomainProviderHandler(providerService)
	domainHandler := handlers.NewDomainHandler(domainService, providerService)
	recordHandler := handlers.NewDnsRecordHandler(recordService, domainService)

	return &Module{
		Base:            module.NewBase(ModuleName, b),
		providerHandler: providerHandler,
		domainHandler:   domainHandler,
		recordHandler:   recordHandler,

		providerService: providerService,
		domainService:   domainService,
		recordService:   recordService,

		providerRepo:  providerRepo,
		domainRepo:    domainRepo,
		dnsRecordRepo: dnsRecordRepo,
	}
}

// RegisterRoutes registers all DNS module routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	RegisterRoutes(router, authMiddleware, m.providerHandler, m.domainHandler, m.recordHandler)
}

// GetProviderService returns the domain provider service
func (m *Module) GetProviderService() *services.DomainProviderService {
	return m.providerService
}

// GetDomainService returns the domain service
func (m *Module) GetDomainService() *services.DomainService {
	return m.domainService
}

// GetRecordService returns the DNS record service
func (m *Module) GetRecordService() *services.DnsRecordService {
	return m.recordService
}

// GetProviderRepository returns the domain provider repository
func (m *Module) GetProviderRepository() *repositories.DomainProviderRepository {
	return m.providerRepo
}

// GetDomainRepository returns the domain repository
func (m *Module) GetDomainRepository() *repositories.DomainRepository {
	return m.domainRepo
}

// GetRecordRepository returns the DNS record repository
func (m *Module) GetRecordRepository() *repositories.DnsRecordRepository {
	return m.dnsRecordRepo
}
