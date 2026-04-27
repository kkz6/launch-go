package services

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	dnstypes "github.com/kkz6/launch-go/internal/modules/dns/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// DomainProviderService handles business logic for domain providers.
type DomainProviderService struct {
	*BaseService
}

// NewDomainProviderService creates a new DomainProviderService instance.
func NewDomainProviderService(deps *ServiceDeps) *DomainProviderService {
	return &DomainProviderService{BaseService: NewBaseService(deps)}
}

// CreateProvider creates a new DNS provider and returns the response DTO.
// A new provider always has zero domains, so the count is rendered as 0
// without a follow-up query.
func (s *DomainProviderService) CreateProvider(ctx context.Context, teamID, userID string, req *dto.CreateDomainProviderRequest) (dto.DomainProviderResponse, error) {
	providerType, err := dnstypes.ParseDNSProvider(req.Provider)
	if err != nil {
		return dto.DomainProviderResponse{}, err
	}

	credentials := map[string]string{"token": req.Token}

	var additionalData map[string]interface{}
	if req.AccountID != "" {
		additionalData = map[string]interface{}{"account_id": req.AccountID}
	}

	provider, err := providers.NewProvider(providers.DNSProviderType(providerType), credentials, additionalData)
	if err != nil {
		return dto.DomainProviderResponse{}, err
	}

	if err := provider.ValidateCredentials(ctx); err != nil {
		s.Logger.Error().Err(err).Str("provider", req.Provider).Msg("Failed to validate credentials")
		return dto.DomainProviderResponse{}, ErrInvalidCredentials
	}

	dp := &models.DomainProvider{
		Profile:        &req.Profile,
		Provider:       providerType,
		Connected:      true,
		Credentials:    credentials,
		AdditionalData: additionalData,
	}
	dp.UserID = userID
	dp.TeamID = teamID

	if err := s.Repos().Provider().Create(ctx, dp); err != nil {
		return dto.DomainProviderResponse{}, err
	}

	return dto.ToDomainProviderResponse(dp, 0), nil
}

// GetProvider retrieves a DNS provider by ID.
func (s *DomainProviderService) GetProvider(ctx context.Context, id, teamID string) (*models.DomainProvider, error) {
	provider, err := s.Repos().Provider().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return nil, notFoundAs(err, "Provider not found")
	}
	return provider, nil
}

// ListProviders lists all DNS providers for a team with their domain counts.
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

// DeleteProvider deletes a DNS provider after verifying it has no domains.
// userID is part of the framework-mutation convention.
func (s *DomainProviderService) DeleteProvider(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	if _, err := s.Repos().Provider().FindByIDAndTeam(ctx, id, teamID); err != nil {
		return notFoundAs(err, "Provider not found")
	}

	count, err := s.Repos().Provider().CountDomainsByProvider(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrProviderHasActiveDomains
	}
	return s.Repos().Provider().Delete(ctx, id)
}

// CheckProviderConnectivity validates a provider's stored credentials and
// updates its connected flag. Validation failures are flattened to a
// generic 400 so credential-shaped errors are not leaked. userID is part
// of the framework-mutation convention.
func (s *DomainProviderService) CheckProviderConnectivity(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	dp, err := s.Repos().Provider().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return notFoundAs(err, "Provider not found")
	}

	provider, err := providers.NewProvider(providers.DNSProviderType(dp.Provider), dp.Credentials, dp.AdditionalData)
	if err != nil {
		return fiberutil.BadRequest("Provider connectivity check failed")
	}

	if err := provider.ValidateCredentials(ctx); err != nil {
		s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{"connected": false})
		return fiberutil.BadRequest("Provider connectivity check failed")
	}

	s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{"connected": true})
	return nil
}

// SyncDomains synchronizes domains from a provider into the local database.
// Provider-side failures are flattened to a generic 400 so credential-
// shaped errors are not leaked.
func (s *DomainProviderService) SyncDomains(ctx context.Context, id, teamID, userID string) error {
	dp, err := s.Repos().Provider().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return notFoundAs(err, "Provider not found")
	}

	s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{
		"sync_status":        dnstypes.SyncStatusSyncing,
		"sync_error_message": nil,
	})

	provider, err := providers.NewProvider(providers.DNSProviderType(dp.Provider), dp.Credentials, dp.AdditionalData)
	if err != nil {
		s.markSyncFailed(ctx, id, err.Error())
		return fiberutil.BadRequest("Failed to sync domains")
	}

	domainsList, err := provider.ListDomains(ctx)
	if err != nil {
		s.markSyncFailed(ctx, id, err.Error())
		return fiberutil.BadRequest("Failed to sync domains")
	}

	err = s.Repos().Provider().Transaction(ctx, func(tx *gorm.DB) error {
		for providerID, domainName := range domainsList {
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

			provider.SetDomain(domainName)
			records, err := provider.ListRecords(ctx)
			if err != nil {
				s.Logger.Warn().Err(err).Str("domain", domainName).Msg("Failed to list records for domain")
				continue
			}

			for _, r := range records {
				pr := fromProviderRecord(r)
				_, err := s.Repos().DNSRecord().UpdateOrCreate(ctx, map[string]interface{}{
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
					s.Logger.Warn().Err(err).Str("domain", domainName).Str("record", r.Name).Msg("Failed to sync record")
				}
			}
		}
		return nil
	})

	if err != nil {
		s.markSyncFailed(ctx, id, err.Error())
		return fiberutil.BadRequest("Failed to sync domains")
	}

	s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{
		"sync_status":        dnstypes.SyncStatusCompleted,
		"last_synced_at":     util.NewULID(),
		"sync_error_message": nil,
	})
	return nil
}

// CountDomainsByProvider counts domains for a provider.
func (s *DomainProviderService) CountDomainsByProvider(ctx context.Context, providerID string) (int64, error) {
	return s.Repos().Provider().CountDomainsByProvider(ctx, providerID)
}

func (s *DomainProviderService) markSyncFailed(ctx context.Context, id, errMsg string) {
	s.Repos().Provider().UpdateFields(ctx, id, map[string]interface{}{
		"sync_status":        dnstypes.SyncStatusFailed,
		"sync_error_message": errMsg,
	})
}

// fromProviderRecord converts a providers.ProviderRecord to models.ProviderRecord.
func fromProviderRecord(r providers.ProviderRecord) models.ProviderRecord {
	return models.ProviderRecord{
		ID:       r.ID,
		Type:     dnstypes.RecordType(r.Type),
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
