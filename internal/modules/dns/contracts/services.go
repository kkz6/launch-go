package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
)

// DomainProviderService defines the interface for domain provider service operations
type DomainProviderService interface {
	CreateProvider(ctx context.Context, userID, teamID string, req *dto.CreateDomainProviderRequest) (*models.DomainProvider, error)
	GetProvider(ctx context.Context, id, teamID string) (*models.DomainProvider, error)
	ListProviders(ctx context.Context, teamID string) ([]dto.DomainProviderResponse, error)
	DeleteProvider(ctx context.Context, id, teamID string) error
	CheckProviderConnectivity(ctx context.Context, id, teamID string) error
	SyncDomains(ctx context.Context, id, userID, teamID string) error
	CountDomainsByProvider(ctx context.Context, providerID string) (int64, error)
}

// DomainService defines the interface for domain service operations
type DomainService interface {
	CreateDomain(ctx context.Context, userID, teamID string, req *dto.CreateDomainRequest) (*models.Domain, error)
	GetDomain(ctx context.Context, id, teamID string) (*models.Domain, error)
	ListDomains(ctx context.Context, teamID string) ([]dto.DomainResponse, error)
	DeleteDomain(ctx context.Context, id, teamID string, deleteFromProvider bool) error
	GetDomainRecords(ctx context.Context, domainID, teamID string) ([]dto.DNSRecordResponse, error)
}

// DNSRecordService defines the interface for DNS record service operations
type DNSRecordService interface {
	CreateRecord(ctx context.Context, domainID, teamID string, req *dto.CreateDNSRecordRequest) (*models.DNSRecord, error)
	UpdateRecord(ctx context.Context, recordID, domainID, teamID string, req *dto.UpdateDNSRecordRequest) (*models.DNSRecord, error)
	DeleteRecord(ctx context.Context, recordID, domainID, teamID string) error
	GetRecordTypes() []string
	// CreateRecordForSite creates a DNS A record for a site, handling subdomain calculation internally
	CreateRecordForSite(ctx context.Context, domainID, teamID, siteAddress, serverIP string) error
}
