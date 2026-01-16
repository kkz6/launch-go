package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// DomainProviderService handles business logic for domain providers
type DomainProviderService struct {
	*BaseService
}

// NewDomainProviderService creates a new DomainProviderService instance
func NewDomainProviderService(deps *ServiceDeps) *DomainProviderService {
	return &DomainProviderService{
		BaseService: NewBaseService(deps),
	}
}

// CreateProvider creates a new DNS provider
func (s *DomainProviderService) CreateProvider(ctx context.Context, userID, teamID string, req *dto.CreateDomainProviderRequest) (*models.DomainProvider, error) {
	providerType, err := enums.ParseDnsProvider(req.Provider)
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
		s.Logger().Error().Err(err).Str("provider", req.Provider).Msg("Failed to validate credentials")
		return nil, ErrInvalidCredentials
	}

	// Create domain provider record
	dp := &models.DomainProvider{
		UserID:         userID,
		TeamID:         &teamID,
		Profile:        &req.Profile,
		Provider:       providerType,
		Connected:      true,
		Credentials:    credentials,
		AdditionalData: additionalData,
	}

	if err := s.Repos().Provider().Create(ctx, dp); err != nil {
		return nil, err
	}

	return dp, nil
}

// GetProvider retrieves a DNS provider by ID
func (s *DomainProviderService) GetProvider(ctx context.Context, id, teamID string) (*models.DomainProvider, error) {
	provider, err := s.Repos().Provider().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}

	return provider, nil
}

// ListProviders lists all DNS providers for a team
func (s *DomainProviderService) ListProviders(ctx context.Context, teamID string) ([]dto.DomainProviderResponse, error) {
	providersList, counts, err := s.Repos().Provider().FindByTeamWithDomainCount(ctx, teamID)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.DomainProviderResponse, len(providersList))
	for i, p := range providersList {
		responses[i] = dto.ToDomainProviderResponse(&p, counts[p.ID])
	}

	return responses, nil
}

// DeleteProvider deletes a DNS provider
func (s *DomainProviderService) DeleteProvider(ctx context.Context, id, teamID string) error {
	_, err := s.Repos().Provider().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProviderNotFound
		}
		return err
	}

	// Check if provider has domains
	count, err := s.Repos().Provider().CountDomainsByProvider(ctx, id)
	if err != nil {
		return err
	}

	if count > 0 {
		return ErrProviderHasActiveDomains
	}

	return s.Repos().Provider().Delete(ctx, id)
}

// CheckProviderConnectivity checks if the provider credentials are still valid
func (s *DomainProviderService) CheckProviderConnectivity(ctx context.Context, id, teamID string) error {
	dp, err := s.Repos().Provider().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProviderNotFound
		}
		return err
	}

	provider, err := providers.NewProvider(providers.DnsProviderType(dp.Provider), dp.Credentials, dp.AdditionalData)
	if err != nil {
		return err
	}

	if err := provider.ValidateCredentials(ctx); err != nil {
		s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{
			"connected": false,
		})
		return err
	}

	s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{
		"connected": true,
	})

	return nil
}

// SyncDomains synchronizes domains from a provider
func (s *DomainProviderService) SyncDomains(ctx context.Context, id, userID, teamID string) error {
	dp, err := s.Repos().Provider().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProviderNotFound
		}
		return err
	}

	// Update sync status
	s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{
		"sync_status":        enums.SyncStatusSyncing,
		"sync_error_message": nil,
	})

	provider, err := providers.NewProvider(providers.DnsProviderType(dp.Provider), dp.Credentials, dp.AdditionalData)
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
	err = s.Repos().Provider().WithTransaction(ctx, func(tx *gorm.DB) error {
		for providerID, domainName := range domainsList {
			// Create or update domain
			domain, err := s.Repos().Domain().UpdateOrCreate(ctx, map[string]interface{}{
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
				s.Logger().Warn().Err(err).Str("domain", domainName).Msg("Failed to list records for domain")
				continue
			}

			for _, r := range records {
				pr := fromProviderRecord(r)
				_, err := s.Repos().DnsRecord().UpdateOrCreate(ctx, map[string]interface{}{
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
					s.Logger().Warn().Err(err).Str("domain", domainName).Str("record", r.Name).Msg("Failed to sync record")
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
	s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{
		"sync_status":        enums.SyncStatusCompleted,
		"last_synced_at":     now,
		"sync_error_message": nil,
	})

	return nil
}

// CountDomainsByProvider counts domains for a provider
func (s *DomainProviderService) CountDomainsByProvider(ctx context.Context, providerID string) (int64, error) {
	return s.Repos().Provider().CountDomainsByProvider(ctx, providerID)
}

func (s *DomainProviderService) markSyncFailed(ctx context.Context, id, errMsg string) {
	s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{
		"sync_status":        enums.SyncStatusFailed,
		"sync_error_message": errMsg,
	})
}

// fromProviderRecord converts a providers.ProviderRecord to models.ProviderRecord
func fromProviderRecord(r providers.ProviderRecord) models.ProviderRecord {
	return models.ProviderRecord{
		ID:       r.ID,
		Type:     enums.RecordType(r.Type),
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
