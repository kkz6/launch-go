package contracts

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
)

// DomainProviderRepository defines the interface for domain provider repository operations
type DomainProviderRepository interface {
	Create(ctx context.Context, provider *models.DomainProvider) error
	FindByID(ctx context.Context, id string) (*models.DomainProvider, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.DomainProvider, error)
	FindByTeam(ctx context.Context, teamID string) ([]models.DomainProvider, error)
	FindByTeamWithDomainCount(ctx context.Context, teamID string) ([]models.DomainProvider, map[string]int, error)
	Update(ctx context.Context, provider *models.DomainProvider) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	CountDomainsByProvider(ctx context.Context, providerID string) (int64, error)
}

// DomainRepository defines the interface for domain repository operations
type DomainRepository interface {
	Create(ctx context.Context, domain *models.Domain) error
	FindByID(ctx context.Context, id string) (*models.Domain, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Domain, error)
	FindByTeam(ctx context.Context, teamID string) ([]models.Domain, error)
	FindByProvider(ctx context.Context, providerID string) ([]models.Domain, error)
	FindByAddressAndProvider(ctx context.Context, address, providerID string) (*models.Domain, error)
	Update(ctx context.Context, domain *models.Domain) error
	UpdateOrCreate(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*models.Domain, error)
	Delete(ctx context.Context, id string) error
}

// DnsRecordRepository defines the interface for DNS record repository operations
type DnsRecordRepository interface {
	Create(ctx context.Context, record *models.DnsRecord) error
	FindByID(ctx context.Context, id string) (*models.DnsRecord, error)
	FindByIDAndDomain(ctx context.Context, id, domainID string) (*models.DnsRecord, error)
	FindByDomain(ctx context.Context, domainID string) ([]models.DnsRecord, error)
	FindByType(ctx context.Context, domainID string, recordType enums.RecordType) ([]models.DnsRecord, error)
	Update(ctx context.Context, record *models.DnsRecord) error
	UpdateOrCreate(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*models.DnsRecord, error)
	Delete(ctx context.Context, id string) error
	DeleteByDomain(ctx context.Context, domainID string) error
}

// TransactionManager defines the interface for transaction management
type TransactionManager interface {
	BeginTransaction(ctx context.Context) *gorm.DB
	WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}
