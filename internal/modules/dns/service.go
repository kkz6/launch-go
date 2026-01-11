package dns

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

var (
	ErrProviderNotFound       = errors.New("provider not found")
	ErrDomainNotFound         = errors.New("domain not found")
	ErrRecordNotFound         = errors.New("record not found")
	ErrRecordNotEditable      = errors.New("record cannot be edited")
	ErrRecordNotDeletable     = errors.New("record cannot be deleted")
	ErrProviderHasActiveDomains = errors.New("provider has active domains")
	ErrInvalidCredentials     = errors.New("invalid credentials")
)

// toProviderDnsRecord converts a dns.DnsRecord to providers.DnsRecord
func toProviderDnsRecord(r *DnsRecord) *providers.DnsRecord {
	return &providers.DnsRecord{
		ID:         r.ID,
		ProviderID: r.ProviderID,
		Type:       providers.RecordType(r.Type),
		Name:       r.Name,
		Value:      r.Value,
		TTL:        r.TTL,
		Priority:   r.Priority,
		Tag:        r.Tag,
		Weight:     r.Weight,
		Port:       r.Port,
		Flags:      r.Flags,
		Comment:    r.Comment,
		Proxied:    r.Proxied,
	}
}

// fromProviderRecord converts a providers.ProviderRecord to dns.ProviderRecord
func fromProviderRecord(r providers.ProviderRecord) ProviderRecord {
	return ProviderRecord{
		ID:       r.ID,
		Type:     RecordType(r.Type),
		Name:     r.Name,
		Value:    r.Value,
		TTL:      r.TTL,
		Priority: r.Priority,
		Tag:      r.Tag,
		Weight:   r.Weight,
		Port:     r.Port,
		Flags:    r.Flags,
		Comment:  r.Comment,
		Proxied:  r.Proxied,
	}
}

// Service handles business logic for the DNS module
type Service struct {
	repo   *Repository
	logger *zerolog.Logger
}

// NewService creates a new Service instance
func NewService(repo *Repository, logger *zerolog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// Domain Provider Methods

// CreateProvider creates a new DNS provider
func (s *Service) CreateProvider(ctx context.Context, userID, teamID string, req *CreateDomainProviderRequest) (*DomainProvider, error) {
	providerType, err := ParseDnsProvider(req.Provider)
	if err != nil {
		return nil, err
	}

	// Create credentials map
	credentials := map[string]string{
		"token": req.Token,
	}

	// Build additional data
	var additionalData map[string]interface{}
	if req.AccountID != "" {
		additionalData = map[string]interface{}{
			"account_id": req.AccountID,
		}
	}

	// Create provider instance to validate credentials
	provider, err := providers.NewProvider(providers.DnsProviderType(providerType), credentials, additionalData)
	if err != nil {
		return nil, err
	}

	// Validate credentials
	if err := provider.ValidateCredentials(ctx); err != nil {
		s.logger.Error().Err(err).Str("provider", req.Provider).Msg("Failed to validate credentials")
		return nil, ErrInvalidCredentials
	}

	// Create domain provider record
	dp := &DomainProvider{
		UserID:    userID,
		TeamID:    teamID,
		Profile:   req.Profile,
		Provider:  providerType,
		Connected: true,
	}

	if err := dp.SetCredentials(credentials); err != nil {
		return nil, err
	}

	if additionalData != nil {
		if err := dp.SetAdditionalData(additionalData); err != nil {
			return nil, err
		}
	}

	if err := s.repo.CreateDomainProvider(ctx, dp); err != nil {
		return nil, err
	}

	return dp, nil
}

// GetProvider retrieves a DNS provider by ID
func (s *Service) GetProvider(ctx context.Context, id, teamID string) (*DomainProvider, error) {
	provider, err := s.repo.FindDomainProviderByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return provider, nil
}

// ListProviders lists all DNS providers for a team
func (s *Service) ListProviders(ctx context.Context, teamID string) ([]DomainProviderResponse, error) {
	providersList, counts, err := s.repo.FindDomainProvidersByTeamWithDomainCount(ctx, teamID)
	if err != nil {
		return nil, err
	}

	responses := make([]DomainProviderResponse, len(providersList))
	for i, p := range providersList {
		responses[i] = ToDomainProviderResponse(&p, counts[p.ID])
	}

	return responses, nil
}

// DeleteProvider deletes a DNS provider
func (s *Service) DeleteProvider(ctx context.Context, id, teamID string) error {
	provider, err := s.repo.FindDomainProviderByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProviderNotFound
		}
		return err
	}

	// Check if provider has domains
	count, err := s.repo.CountDomainsByProvider(ctx, id)
	if err != nil {
		return err
	}

	if count > 0 {
		return ErrProviderHasActiveDomains
	}

	return s.repo.DeleteDomainProvider(ctx, provider.ID)
}

// CheckProviderConnectivity checks if the provider credentials are still valid
func (s *Service) CheckProviderConnectivity(ctx context.Context, id, teamID string) error {
	dp, err := s.repo.FindDomainProviderByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProviderNotFound
		}
		return err
	}

	credentials, err := dp.GetCredentials()
	if err != nil {
		return err
	}

	additionalData, err := dp.GetAdditionalData()
	if err != nil {
		return err
	}

	provider, err := providers.NewProvider(providers.DnsProviderType(dp.Provider), credentials, additionalData)
	if err != nil {
		return err
	}

	if err := provider.ValidateCredentials(ctx); err != nil {
		s.repo.UpdateDomainProviderFields(ctx, id, map[string]interface{}{
			"connected": false,
		})
		return err
	}

	s.repo.UpdateDomainProviderFields(ctx, id, map[string]interface{}{
		"connected": true,
	})

	return nil
}

// SyncDomains synchronizes domains from a provider
func (s *Service) SyncDomains(ctx context.Context, id, userID, teamID string) error {
	dp, err := s.repo.FindDomainProviderByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProviderNotFound
		}
		return err
	}

	// Update sync status
	s.repo.UpdateDomainProviderFields(ctx, id, map[string]interface{}{
		"sync_status":        SyncStatusSyncing,
		"sync_error_message": nil,
	})

	credentials, err := dp.GetCredentials()
	if err != nil {
		s.markSyncFailed(ctx, id, err.Error())
		return err
	}

	additionalData, err := dp.GetAdditionalData()
	if err != nil {
		s.markSyncFailed(ctx, id, err.Error())
		return err
	}

	provider, err := providers.NewProvider(providers.DnsProviderType(dp.Provider), credentials, additionalData)
	if err != nil {
		s.markSyncFailed(ctx, id, err.Error())
		return err
	}

	// List domains from provider
	domainsList, err := provider.ListDomains(ctx)
	if err != nil {
		s.markSyncFailed(ctx, id, err.Error())
		return err
	}

	// Sync domains within a transaction
	err = s.repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		for providerID, domainName := range domainsList {
			// Create or update domain
			domain, err := s.repo.UpdateOrCreateDomain(ctx, map[string]interface{}{
				"provider_id":        providerID,
				"domain_provider_id": dp.ID,
				"address":            domainName,
				"team_id":            teamID,
			}, map[string]interface{}{
				"user_id": userID,
				"label":   domainName,
			})
			if err != nil {
				return err
			}

			// Sync records for this domain
			provider.SetDomain(domainName)
			records, err := provider.ListRecords(ctx)
			if err != nil {
				s.logger.Warn().Err(err).Str("domain", domainName).Msg("Failed to list records for domain")
				continue
			}

			for _, r := range records {
				pr := fromProviderRecord(r)
				_, err := s.repo.UpdateOrCreateDnsRecord(ctx, map[string]interface{}{
					"domain_id":   domain.ID,
					"type":        pr.Type,
					"name":        pr.Name,
					"provider_id": pr.ID,
				}, map[string]interface{}{
					"value":    pr.Value,
					"ttl":      pr.TTL,
					"priority": pr.Priority,
					"tag":      pr.Tag,
					"weight":   pr.Weight,
					"port":     pr.Port,
					"flags":    pr.Flags,
					"proxied":  pr.Proxied,
				})
				if err != nil {
					s.logger.Warn().Err(err).Str("domain", domainName).Str("record", r.Name).Msg("Failed to sync record")
				}
			}
		}
		return nil
	})

	if err != nil {
		s.markSyncFailed(ctx, id, err.Error())
		return err
	}

	// Update sync status to completed
	now := utils.NewULID() // Using ULID for timestamp as a workaround
	s.repo.UpdateDomainProviderFields(ctx, id, map[string]interface{}{
		"sync_status":        SyncStatusCompleted,
		"last_synced_at":     now,
		"sync_error_message": nil,
	})

	return nil
}

func (s *Service) markSyncFailed(ctx context.Context, id, errMsg string) {
	s.repo.UpdateDomainProviderFields(ctx, id, map[string]interface{}{
		"sync_status":        SyncStatusFailed,
		"sync_error_message": errMsg,
	})
}

// Domain Methods

// CreateDomain creates a new domain
func (s *Service) CreateDomain(ctx context.Context, userID, teamID string, req *CreateDomainRequest) (*Domain, error) {
	// Get the provider
	provider, err := s.repo.FindDomainProviderByIDAndTeam(ctx, req.Provider, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}

	credentials, err := provider.GetCredentials()
	if err != nil {
		return nil, err
	}

	additionalData, err := provider.GetAdditionalData()
	if err != nil {
		return nil, err
	}

	dnsProvider, err := providers.NewProvider(providers.DnsProviderType(provider.Provider), credentials, additionalData)
	if err != nil {
		return nil, err
	}

	var domain *Domain
	err = s.repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		// Add domain to provider
		providerID, err := dnsProvider.AddDomain(ctx, req.Address)
		if err != nil {
			return err
		}

		// Create domain record
		domain = &Domain{
			UserID:           userID,
			TeamID:           teamID,
			DomainProviderID: provider.ID,
			ProviderID:       providerID,
			Label:            req.Label,
			Address:          req.Address,
		}

		if err := s.repo.CreateDomain(ctx, domain); err != nil {
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
			nsRecord := &DnsRecord{
				DomainID:   domain.ID,
				ProviderID: fmt.Sprintf("ns-%d", i),
				Type:       RecordTypeNS,
				Name:       "@",
				Value:      ns,
				TTL:        3600,
			}
			if err := s.repo.CreateDnsRecord(ctx, nsRecord); err != nil {
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
func (s *Service) GetDomain(ctx context.Context, id, teamID string) (*Domain, error) {
	domain, err := s.repo.FindDomainByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return domain, nil
}

// ListDomains lists all domains for a team
func (s *Service) ListDomains(ctx context.Context, teamID string) ([]DomainResponse, error) {
	domains, err := s.repo.FindDomainsByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	responses := make([]DomainResponse, len(domains))
	for i, d := range domains {
		responses[i] = ToDomainResponse(&d)
	}

	return responses, nil
}

// DeleteDomain deletes a domain
func (s *Service) DeleteDomain(ctx context.Context, id, teamID string, deleteFromProvider bool) error {
	domain, err := s.repo.FindDomainByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDomainNotFound
		}
		return err
	}

	if deleteFromProvider && domain.Provider != nil {
		credentials, err := domain.Provider.GetCredentials()
		if err == nil {
			additionalData, _ := domain.Provider.GetAdditionalData()
			dnsProvider, err := providers.NewProvider(providers.DnsProviderType(domain.Provider.Provider), credentials, additionalData)
			if err == nil {
				if err := dnsProvider.DeleteDomain(ctx, domain.Address); err != nil {
					s.logger.Warn().Err(err).Str("domain", domain.Address).Msg("Failed to delete domain from provider")
				}
			}
		}
	}

	// Delete records first
	if err := s.repo.DeleteDnsRecordsByDomain(ctx, id); err != nil {
		return err
	}

	return s.repo.DeleteDomain(ctx, id)
}

// GetDomainRecords retrieves all DNS records for a domain
func (s *Service) GetDomainRecords(ctx context.Context, domainID, teamID string) ([]DnsRecordResponse, error) {
	domain, err := s.repo.FindDomainByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	records, err := s.repo.FindDnsRecordsByDomain(ctx, domain.ID)
	if err != nil {
		return nil, err
	}

	responses := make([]DnsRecordResponse, len(records))
	for i, r := range records {
		responses[i] = ToDnsRecordResponse(&r)
	}

	return responses, nil
}

// DNS Record Methods

// CreateRecord creates a new DNS record
func (s *Service) CreateRecord(ctx context.Context, domainID, teamID string, req *CreateDnsRecordRequest) (*DnsRecord, error) {
	domain, err := s.repo.FindDomainByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	record := req.ToModel(domainID)

	// Get provider to add record
	credentials, err := domain.Provider.GetCredentials()
	if err != nil {
		return nil, err
	}

	additionalData, err := domain.Provider.GetAdditionalData()
	if err != nil {
		return nil, err
	}

	dnsProvider, err := providers.NewProvider(providers.DnsProviderType(domain.Provider.Provider), credentials, additionalData)
	if err != nil {
		return nil, err
	}

	dnsProvider.SetDomain(domain.Address)

	err = s.repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		// Add record to provider
		providerID, err := dnsProvider.AddRecord(ctx, toProviderDnsRecord(record))
		if err != nil {
			return err
		}

		record.ProviderID = providerID
		return s.repo.CreateDnsRecord(ctx, record)
	})

	if err != nil {
		return nil, err
	}

	return record, nil
}

// UpdateRecord updates a DNS record
func (s *Service) UpdateRecord(ctx context.Context, recordID, domainID, teamID string, req *UpdateDnsRecordRequest) (*DnsRecord, error) {
	domain, err := s.repo.FindDomainByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	record, err := s.repo.FindDnsRecordByIDAndDomain(ctx, recordID, domainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	if !record.IsEditable() {
		return nil, ErrRecordNotEditable
	}

	// Get provider
	credentials, err := domain.Provider.GetCredentials()
	if err != nil {
		return nil, err
	}

	additionalData, err := domain.Provider.GetAdditionalData()
	if err != nil {
		return nil, err
	}

	dnsProvider, err := providers.NewProvider(providers.DnsProviderType(domain.Provider.Provider), credentials, additionalData)
	if err != nil {
		return nil, err
	}

	dnsProvider.SetDomain(domain.Address)

	// Apply updates
	req.ApplyToModel(record)

	err = s.repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		// Update record at provider
		if err := dnsProvider.UpdateRecord(ctx, toProviderDnsRecord(record)); err != nil {
			return err
		}

		return s.repo.UpdateDnsRecord(ctx, record)
	})

	if err != nil {
		return nil, err
	}

	return record, nil
}

// DeleteRecord deletes a DNS record
func (s *Service) DeleteRecord(ctx context.Context, recordID, domainID, teamID string) error {
	domain, err := s.repo.FindDomainByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDomainNotFound
		}
		return err
	}

	record, err := s.repo.FindDnsRecordByIDAndDomain(ctx, recordID, domainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRecordNotFound
		}
		return err
	}

	if !record.IsDeletable() {
		return ErrRecordNotDeletable
	}

	// Get provider
	credentials, err := domain.Provider.GetCredentials()
	if err != nil {
		return err
	}

	additionalData, err := domain.Provider.GetAdditionalData()
	if err != nil {
		return err
	}

	dnsProvider, err := providers.NewProvider(providers.DnsProviderType(domain.Provider.Provider), credentials, additionalData)
	if err != nil {
		return err
	}

	dnsProvider.SetDomain(domain.Address)

	return s.repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		// Delete record from provider
		if err := dnsProvider.DeleteRecord(ctx, toProviderDnsRecord(record)); err != nil {
			return err
		}

		return s.repo.DeleteDnsRecord(ctx, record.ID)
	})
}

// GetRecordTypes returns all available record types
func (s *Service) GetRecordTypes() []string {
	types := AllRecordTypes()
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = t.String()
	}
	return result
}
