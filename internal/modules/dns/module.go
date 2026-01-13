package dns

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/handlers"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module represents the DNS module
type Module struct {
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

// NewModuleFromContext creates a new DNS module from app context
func NewModuleFromContext(ctx *app.Context) *Module {
	return NewModule(ctx.DB, ctx.Logger)
}

// Name returns the module name (implements app.Module)
func (m *Module) Name() string {
	return "dns"
}

// NewModule creates a new DNS module instance
func NewModule(db *gorm.DB, logger *zerolog.Logger) *Module {
	// Initialize repositories
	providerRepo := repositories.NewDomainProviderRepository(db)
	domainRepo := repositories.NewDomainRepository(db)
	dnsRecordRepo := repositories.NewDnsRecordRepository(db)

	// Initialize services
	providerService := services.NewDomainProviderService(providerRepo, domainRepo, dnsRecordRepo, logger)
	domainService := services.NewDomainService(providerRepo, domainRepo, dnsRecordRepo, logger)
	recordService := services.NewDnsRecordService(domainRepo, dnsRecordRepo, logger)

	// Initialize handlers
	providerHandler := handlers.NewDomainProviderHandler(providerService)
	domainHandler := handlers.NewDomainHandler(domainService, providerService)
	recordHandler := handlers.NewDnsRecordHandler(recordService, domainService)

	return &Module{
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

// AutoMigrate runs database migrations for the DNS module
func (m *Module) AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.DomainProvider{},
		&models.Domain{},
		&models.DnsRecord{},
	)
}
