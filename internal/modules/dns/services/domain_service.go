package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
)

// DomainService handles business logic for domains
type DomainService struct {
	providerRepo  *repositories.DomainProviderRepository
	domainRepo    *repositories.DomainRepository
	dnsRecordRepo *repositories.DnsRecordRepository
	logger        *zerolog.Logger
}

// NewDomainService creates a new DomainService instance
func NewDomainService(
	providerRepo *repositories.DomainProviderRepository,
	domainRepo *repositories.DomainRepository,
	dnsRecordRepo *repositories.DnsRecordRepository,
	logger *zerolog.Logger,
) *DomainService {
	return &DomainService{
		providerRepo:  providerRepo,
		domainRepo:    domainRepo,
		dnsRecordRepo: dnsRecordRepo,
		logger:        logger,
	}
}

// CreateDomain creates a new domain
func (s *DomainService) CreateDomain(ctx context.Context, userID, teamID string, req *dto.CreateDomainRequest) (*models.Domain, error) {
	// Get the provider
	provider, err := s.providerRepo.FindByIDAndTeam(ctx, req.Provider, teamID)
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
	err = s.domainRepo.WithTransaction(ctx, func(tx *gorm.DB) error {
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

		if err := s.domainRepo.Create(ctx, domain); err != nil {
			return err
		}

		// Get nameservers and create NS records
		dnsProvider.SetDomain(req.Address)
		nameservers, err := dnsProvider.GetNameservers(ctx)
		if err != nil {
			s.logger.Warn().Err(err).Str("domain", req.Address).Msg("Failed to get nameservers")
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
			if err := s.dnsRecordRepo.Create(ctx, nsRecord); err != nil {
				s.logger.Warn().Err(err).Str("ns", ns).Msg("Failed to create NS record")
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
	domain, err := s.domainRepo.FindByIDAndTeam(ctx, id, teamID)
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
	domain, err := s.domainRepo.FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	domain.Label = req.Label

	if err := s.domainRepo.Update(ctx, domain); err != nil {
		return nil, err
	}

	return domain, nil
}

// ListDomains lists all domains for a team
func (s *DomainService) ListDomains(ctx context.Context, teamID string) ([]dto.DomainResponse, error) {
	domains, err := s.domainRepo.FindByTeam(ctx, teamID)
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
	domain, err := s.domainRepo.FindByIDAndTeam(ctx, id, teamID)
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
				s.logger.Warn().Err(err).Str("domain", domain.Address).Msg("Failed to delete domain from provider")
			}
		}
	}

	// Delete records first
	if err := s.dnsRecordRepo.DeleteByDomain(ctx, id); err != nil {
		return err
	}

	return s.domainRepo.Delete(ctx, id)
}

// GetDomainRecords retrieves all DNS records for a domain
func (s *DomainService) GetDomainRecords(ctx context.Context, domainID, teamID string) ([]dto.DnsRecordResponse, error) {
	domain, err := s.domainRepo.FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	records, err := s.dnsRecordRepo.FindByDomain(ctx, domain.ID)
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
