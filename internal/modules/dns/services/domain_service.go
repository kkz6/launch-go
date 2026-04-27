package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	dnstypes "github.com/kkz6/launch-go/internal/modules/dns/types"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// DomainService handles business logic for domains.
type DomainService struct {
	*BaseService
}

// NewDomainService creates a new DomainService instance.
func NewDomainService(deps *ServiceDeps) *DomainService {
	return &DomainService{BaseService: NewBaseService(deps)}
}

// CreateDomain creates a new domain and returns the response DTO.
func (s *DomainService) CreateDomain(ctx context.Context, teamID, userID string, req *dto.CreateDomainRequest) (dto.DomainResponse, error) {
	domain, err := s.buildAndDispatchDomain(ctx, teamID, userID, req)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	return dto.ToDomainResponse(domain), nil
}

// createDomain runs the create flow and returns the created model.
func (s *DomainService) buildAndDispatchDomain(ctx context.Context, teamID, userID string, req *dto.CreateDomainRequest) (*models.Domain, error) {
	provider, err := s.Repos().Provider().FindByIDAndTeam(ctx, req.Provider, teamID)
	if err != nil {
		return nil, notFoundAs(err, "Provider not found")
	}

	dnsProvider, err := providers.NewProvider(providers.DNSProviderType(provider.Provider), provider.Credentials, provider.AdditionalData)
	if err != nil {
		return nil, err
	}

	providerID, err := dnsProvider.AddDomain(ctx, req.Address)
	if err != nil {
		return nil, wrapProviderErr(err)
	}

	domain := &models.Domain{
		DomainProviderID: provider.ID,
		ProviderID:       providerID,
		Label:            req.Label,
		Address:          req.Address,
	}
	domain.UserID = userID
	domain.TeamID = teamID

	err = s.Repos().Domain().Transaction(ctx, func(tx *gorm.DB) error {
		if err := s.Repos().Domain().Create(ctx, domain); err != nil {
			return err
		}

		dnsProvider.SetDomain(req.Address)
		nameservers, err := dnsProvider.GetNameservers(ctx)
		if err != nil {
			s.Logger.Warn().Err(err).Str("domain", req.Address).Msg("Failed to get nameservers")
			return nil
		}

		for i, ns := range nameservers {
			nsRecord := &models.DNSRecord{
				DomainID:   domain.ID,
				ProviderID: fmt.Sprintf("ns-%d", i),
				Type:       dnstypes.RecordTypeNS,
				Name:       "@",
				Value:      ns,
				TTL:        3600,
			}
			nsRecord.TeamID = teamID
			if err := s.Repos().DNSRecord().Create(ctx, nsRecord); err != nil {
				s.Logger.Warn().Err(err).Str("ns", ns).Msg("Failed to create NS record")
			}
		}

		return nil
	})

	if err != nil {
		// Compensating action: remove the domain from the provider.
		if deleteErr := dnsProvider.DeleteDomain(ctx, req.Address); deleteErr != nil {
			s.Logger.Error().Err(deleteErr).Str("domain", req.Address).Msg("Failed to delete domain from provider after DB transaction failure")
		}
		return nil, err
	}

	return domain, nil
}

// GetDomain retrieves a domain by ID.
func (s *DomainService) GetDomain(ctx context.Context, id, teamID string) (*models.Domain, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return nil, notFoundAs(err, "Domain not found")
	}
	return domain, nil
}

// ListDomainsPage returns the index-page payload combining domains and
// providers for the team.
func (s *DomainService) ListDomainsPage(ctx context.Context, teamID string) (dto.DomainIndexPageData, error) {
	domains, err := s.ListDomains(ctx, teamID)
	if err != nil {
		return dto.DomainIndexPageData{}, err
	}
	providerList, err := s.Services().Provider().ListProviders(ctx, teamID)
	if err != nil {
		return dto.DomainIndexPageData{}, err
	}
	return dto.DomainIndexPageData{Domains: domains, Providers: providerList}, nil
}

// GetDomainPage returns the show-page payload for a single domain.
func (s *DomainService) GetDomainPage(ctx context.Context, id, teamID string) (dto.DomainShowPageData, error) {
	domain, err := s.GetDomain(ctx, id, teamID)
	if err != nil {
		return dto.DomainShowPageData{}, err
	}

	records, err := s.GetDomainRecords(ctx, id, teamID)
	if err != nil {
		return dto.DomainShowPageData{}, err
	}

	recordTypeOptions := pkgdto.MapSliceValue(GetRecordTypes(), func(rt string) dto.RecordTypeOption {
		return dto.RecordTypeOption{Value: rt, Label: rt}
	})

	nameservers := make([]string, 0)
	for _, r := range domain.Records {
		if r.Type == dnstypes.RecordTypeNS {
			nameservers = append(nameservers, r.Value)
		}
	}

	var providerResponse *dto.DomainProviderResponse
	if domain.Provider != nil {
		pr := dto.ToDomainProviderResponse(domain.Provider, 0)
		providerResponse = &pr
	}

	return dto.DomainShowPageData{
		Domain:      dto.ToDomainResponse(domain),
		Records:     records,
		RecordTypes: recordTypeOptions,
		Nameservers: nameservers,
		Provider:    providerResponse,
	}, nil
}

// UpdateDomain updates a domain and returns the response DTO. userID is
// part of the framework-mutation convention; not currently audit-logged.
func (s *DomainService) UpdateDomain(ctx context.Context, id, teamID, userID string, req *dto.UpdateDomainRequest) (dto.DomainResponse, error) {
	_ = userID
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return dto.DomainResponse{}, notFoundAs(err, "Domain not found")
	}

	domain.Label = req.Label
	if err := s.Repos().Domain().Update(ctx, domain); err != nil {
		return dto.DomainResponse{}, err
	}
	return dto.ToDomainResponse(domain), nil
}

// ListDomains lists all domains for a team.
func (s *DomainService) ListDomains(ctx context.Context, teamID string) ([]dto.DomainResponse, error) {
	domains, err := s.Repos().Domain().FindByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	return pkgdto.TransformSlice(domains, dto.ToDomainResponse), nil
}

// DeleteDomain deletes a domain. When deleteFromProvider is true and the
// domain has a configured provider, the domain is also removed at the
// provider before being deleted locally. userID is part of the
// framework-mutation convention.
func (s *DomainService) DeleteDomain(ctx context.Context, id, teamID, userID string, deleteFromProvider bool) error {
	_ = userID
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return notFoundAs(err, "Domain not found")
	}

	if deleteFromProvider && domain.Provider != nil {
		if dnsProvider, perr := providers.NewProvider(providers.DNSProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData); perr == nil {
			if err := dnsProvider.DeleteDomain(ctx, domain.Address); err != nil {
				s.Logger.Warn().Err(err).Str("domain", domain.Address).Msg("Failed to delete domain from provider")
			}
		}
	}

	return s.Repos().Domain().Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("domain_id = ?", id).Delete(&models.DNSRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&models.Domain{}).Error
	})
}

// GetDomainRecords retrieves all DNS records for a domain.
func (s *DomainService) GetDomainRecords(ctx context.Context, domainID, teamID string) ([]dto.DNSRecordResponse, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		return nil, notFoundAs(err, "Domain not found")
	}

	records, err := s.Repos().DNSRecord().FindByDomain(ctx, domain.ID)
	if err != nil {
		return nil, err
	}
	return pkgdto.TransformSlice(records, dto.ToDNSRecordResponse), nil
}

// GetDomainNameservers retrieves nameservers for a domain from its provider.
func (s *DomainService) GetDomainNameservers(ctx context.Context, domain *models.Domain) ([]string, error) {
	if domain.Provider == nil {
		return nil, nil
	}

	dnsProvider, err := providers.NewProvider(
		providers.DNSProviderType(domain.Provider.Provider),
		domain.Provider.Credentials,
		domain.Provider.AdditionalData,
	)
	if err != nil {
		return nil, err
	}

	dnsProvider.SetDomain(domain.Address)
	return dnsProvider.GetNameservers(ctx)
}

// SyncDomainRecords syncs DNS records from the provider to the local
// database. userID is part of the framework-mutation convention.
func (s *DomainService) SyncDomainRecords(ctx context.Context, domainID, teamID, userID string) error {
	_ = userID
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		return notFoundAs(err, "Domain not found")
	}

	if domain.Provider == nil {
		return errors.New("domain has no provider configured")
	}

	dnsProvider, err := providers.NewProvider(
		providers.DNSProviderType(domain.Provider.Provider),
		domain.Provider.Credentials,
		domain.Provider.AdditionalData,
	)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	dnsProvider.SetDomain(domain.Address)

	providerRecords, err := dnsProvider.ListRecords(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch records from provider: %w", err)
	}

	records := make([]models.DNSRecord, 0, len(providerRecords))
	for _, pr := range providerRecords {
		record := models.DNSRecord{
			DomainID:   domainID,
			ProviderID: pr.ID,
			Type:       dnstypes.RecordType(pr.Type),
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
		record.TeamID = domain.TeamID
		records = append(records, record)
	}

	return s.Repos().Domain().Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("domain_id = ?", domainID).Delete(&models.DNSRecord{}).Error; err != nil {
			return fmt.Errorf("failed to delete existing records: %w", err)
		}
		if len(records) > 0 {
			if err := tx.CreateInBatches(&records, 100).Error; err != nil {
				return fmt.Errorf("failed to insert records: %w", err)
			}
		}
		return nil
	})
}
