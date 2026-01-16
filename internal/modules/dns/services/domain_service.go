package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
)

// DomainService handles business logic for domains
type DomainService struct {
	*BaseService
}

// NewDomainService creates a new DomainService instance
func NewDomainService(deps *ServiceDeps) *DomainService {
	return &DomainService{
		BaseService: NewBaseService(deps),
	}
}

// CreateDomain creates a new domain
func (s *DomainService) CreateDomain(ctx context.Context, userID, teamID string, req *dto.CreateDomainRequest) (*models.Domain, error) {
	// Get the provider
	provider, err := s.Repos().Provider().FindByIDAndTeam(ctx, req.Provider, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}

	dnsProvider, err := providers.NewProvider(providers.DnsProviderType(provider.Provider), provider.Credentials, provider.AdditionalData)
	if err != nil {
		return nil, err
	}

	var domain *models.Domain
	err = s.Repos().Domain().WithTransaction(ctx, func(tx *gorm.DB) error {
		// Add domain to provider
		providerID, err := dnsProvider.AddDomain(ctx, req.Address)
		if err != nil {
			return err
		}

		// Create domain record
		domain = &models.Domain{
			UserID:           userID,
			TeamID:           &teamID,
			DomainProviderID: provider.ID,
			ProviderID:       providerID,
			Label:            req.Label,
			Address:          req.Address,
		}

		if err := s.Repos().Domain().Create(ctx, domain); err != nil {
			return err
		}

		// Get nameservers and create NS records
		dnsProvider.SetDomain(req.Address)
		nameservers, err := dnsProvider.GetNameservers(ctx)
		if err != nil {
			s.Logger().Warn().Err(err).Str("domain", req.Address).Msg("Failed to get nameservers")
			return nil // Don't fail the whole operation
		}

		for i, ns := range nameservers {
			nsRecord := &models.DnsRecord{
				DomainID:   domain.ID,
				ProviderID: fmt.Sprintf("ns-%d", i),
				Type:       enums.RecordTypeNS,
				Name:       "@",
				Value:      ns,
				TTL:        3600,
			}
			if err := s.Repos().DnsRecord().Create(ctx, nsRecord); err != nil {
				s.Logger().Warn().Err(err).Str("ns", ns).Msg("Failed to create NS record")
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return domain, nil
}

// GetDomain retrieves a domain by ID
func (s *DomainService) GetDomain(ctx context.Context, id, teamID string) (*models.Domain, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	return domain, nil
}

// UpdateDomain updates a domain
func (s *DomainService) UpdateDomain(ctx context.Context, id, teamID string, req *dto.UpdateDomainRequest) (*models.Domain, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	domain.Label = req.Label

	if err := s.Repos().Domain().Update(ctx, domain); err != nil {
		return nil, err
	}

	return domain, nil
}

// ListDomains lists all domains for a team
func (s *DomainService) ListDomains(ctx context.Context, teamID string) ([]dto.DomainResponse, error) {
	domains, err := s.Repos().Domain().FindByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.DomainResponse, len(domains))
	for i, d := range domains {
		responses[i] = dto.ToDomainResponse(&d)
	}

	return responses, nil
}

// DeleteDomain deletes a domain
func (s *DomainService) DeleteDomain(ctx context.Context, id, teamID string, deleteFromProvider bool) error {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDomainNotFound
		}
		return err
	}

	if deleteFromProvider && domain.Provider != nil {
		dnsProvider, err := providers.NewProvider(providers.DnsProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
		if err == nil {
			if err := dnsProvider.DeleteDomain(ctx, domain.Address); err != nil {
				s.Logger().Warn().Err(err).Str("domain", domain.Address).Msg("Failed to delete domain from provider")
			}
		}
	}

	// Delete records first
	if err := s.Repos().DnsRecord().DeleteByDomain(ctx, id); err != nil {
		return err
	}

	return s.Repos().Domain().Delete(ctx, id)
}

// GetDomainRecords retrieves all DNS records for a domain
func (s *DomainService) GetDomainRecords(ctx context.Context, domainID, teamID string) ([]dto.DnsRecordResponse, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	records, err := s.Repos().DnsRecord().FindByDomain(ctx, domain.ID)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.DnsRecordResponse, len(records))
	for i, r := range records {
		responses[i] = dto.ToDnsRecordResponse(&r)
	}

	return responses, nil
}

// GetDomainNameservers retrieves nameservers for a domain from its provider
func (s *DomainService) GetDomainNameservers(ctx context.Context, domain *models.Domain) ([]string, error) {
	if domain.Provider == nil {
		return nil, nil
	}

	dnsProvider, err := providers.NewProvider(
		providers.DnsProviderType(domain.Provider.Provider),
		domain.Provider.Credentials,
		domain.Provider.AdditionalData,
	)
	if err != nil {
		return nil, err
	}

	dnsProvider.SetDomain(domain.Address)

	return dnsProvider.GetNameservers(ctx)
}

// SyncDomainRecords syncs DNS records from the provider to the local database
func (s *DomainService) SyncDomainRecords(ctx context.Context, domainID, teamID string) error {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDomainNotFound
		}
		return err
	}

	if domain.Provider == nil {
		return errors.New("domain has no provider configured")
	}

	// Create provider instance
	dnsProvider, err := providers.NewProvider(
		providers.DnsProviderType(domain.Provider.Provider),
		domain.Provider.Credentials,
		domain.Provider.AdditionalData,
	)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	dnsProvider.SetDomain(domain.Address)

	// Fetch records from provider
	providerRecords, err := dnsProvider.ListRecords(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch records from provider: %w", err)
	}

	// Sync records in a transaction
	return s.Repos().Domain().WithTransaction(ctx, func(tx *gorm.DB) error {
		// Delete existing records for this domain
		if err := s.Repos().DnsRecord().DeleteByDomain(ctx, domainID); err != nil {
			return fmt.Errorf("failed to delete existing records: %w", err)
		}

		// Insert new records from provider
		for _, pr := range providerRecords {
			record := &models.DnsRecord{
				DomainID:   domainID,
				ProviderID: pr.ID,
				Type:       enums.RecordType(pr.Type),
				Name:       pr.Name,
				Value:      pr.Value,
				TTL:        pr.TTL,
				Priority:   pr.Priority,
				Tag:        pr.Tag,
				Weight:     pr.Weight,
				Port:       pr.Port,
				Flags:      pr.Flags,
				Comment:    pr.Comment,
				Proxied:    pr.Proxied,
			}

			if err := s.Repos().DnsRecord().Create(ctx, record); err != nil {
				s.Logger().Warn().Err(err).Str("record", pr.Name).Msg("Failed to create record during sync")
			}
		}

		return nil
	})
}
