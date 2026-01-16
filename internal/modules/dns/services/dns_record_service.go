package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
)

// DnsRecordService handles business logic for DNS records
type DnsRecordService struct {
	*BaseService
}

// NewDnsRecordService creates a new DnsRecordService instance
func NewDnsRecordService(deps *ServiceDeps) *DnsRecordService {
	return &DnsRecordService{
		BaseService: NewBaseService(deps),
	}
}

// CreateRecord creates a new DNS record
func (s *DnsRecordService) CreateRecord(ctx context.Context, domainID, teamID string, req *dto.CreateDnsRecordRequest) (*models.DnsRecord, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	record := req.ToModel(domainID)

	// Get provider to add record
	dnsProvider, err := providers.NewProvider(providers.DnsProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return nil, err
	}

	dnsProvider.SetDomain(domain.Address)

	err = s.Repos().DnsRecord().WithTransaction(ctx, func(tx *gorm.DB) error {
		// Add record to provider
		providerID, err := dnsProvider.AddRecord(ctx, toProviderDnsRecord(record))
		if err != nil {
			return err
		}

		record.ProviderID = providerID

		return s.Repos().DnsRecord().Create(ctx, record)
	})

	if err != nil {
		return nil, err
	}

	return record, nil
}

// UpdateRecord updates a DNS record
func (s *DnsRecordService) UpdateRecord(ctx context.Context, recordID, domainID, teamID string, req *dto.UpdateDnsRecordRequest) (*models.DnsRecord, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	record, err := s.Repos().DnsRecord().FindByIDAndDomain(ctx, recordID, domainID)
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
	dnsProvider, err := providers.NewProvider(providers.DnsProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return nil, err
	}

	dnsProvider.SetDomain(domain.Address)

	// Apply updates
	req.ApplyToModel(record)

	err = s.Repos().DnsRecord().WithTransaction(ctx, func(tx *gorm.DB) error {
		// Update record at provider
		if err := dnsProvider.UpdateRecord(ctx, toProviderDnsRecord(record)); err != nil {
			return err
		}

		return s.Repos().DnsRecord().Update(ctx, record)
	})

	if err != nil {
		return nil, err
	}

	return record, nil
}

// DeleteRecord deletes a DNS record
func (s *DnsRecordService) DeleteRecord(ctx context.Context, recordID, domainID, teamID string) error {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDomainNotFound
		}
		return err
	}

	record, err := s.Repos().DnsRecord().FindByIDAndDomain(ctx, recordID, domainID)
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
	dnsProvider, err := providers.NewProvider(providers.DnsProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return err
	}

	dnsProvider.SetDomain(domain.Address)

	return s.Repos().DnsRecord().WithTransaction(ctx, func(tx *gorm.DB) error {
		// Delete record from provider
		if err := dnsProvider.DeleteRecord(ctx, toProviderDnsRecord(record)); err != nil {
			return err
		}

		return s.Repos().DnsRecord().Delete(ctx, record.ID)
	})
}

// GetRecordTypes returns all available record types
func (s *DnsRecordService) GetRecordTypes() []string {
	return GetRecordTypes()
}

// GetRecordTypes returns all available record types (package-level function)
func GetRecordTypes() []string {
	types := enums.AllRecordTypes()
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = t.String()
	}

	return result
}

// toProviderDnsRecord converts a models.DnsRecord to providers.DnsRecord
func toProviderDnsRecord(r *models.DnsRecord) *providers.DnsRecord {
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
