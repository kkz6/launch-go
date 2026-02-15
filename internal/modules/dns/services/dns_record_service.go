package services

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	dnstypes "github.com/kkz6/launch-go/internal/modules/dns/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DNSRecordService handles business logic for DNS records
type DNSRecordService struct {
	*BaseService
}

// NewDNSRecordService creates a new DNSRecordService instance
func NewDNSRecordService(deps *ServiceDeps) *DNSRecordService {
	return &DNSRecordService{
		BaseService: NewBaseService(deps),
	}
}

// CreateRecord creates a new DNS record
func (s *DNSRecordService) CreateRecord(ctx context.Context, domainID, teamID string, req *dto.CreateDNSRecordRequest) (*models.DNSRecord, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	record := req.ToModel(domainID)
	record.TeamID = domain.TeamID

	// Get provider to add record
	dnsProvider, err := providers.NewProvider(providers.DNSProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return nil, err
	}

	dnsProvider.SetDomain(domain.Address)

	// Add record to provider first
	providerID, err := dnsProvider.AddRecord(ctx, toProviderDNSRecord(record))
	if err != nil {
		return nil, err
	}

	record.ProviderID = providerID

	if err := s.Repos().DNSRecord().Create(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

// UpdateRecord updates a DNS record
func (s *DNSRecordService) UpdateRecord(ctx context.Context, recordID, domainID, teamID string, req *dto.UpdateDNSRecordRequest) (*models.DNSRecord, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	record, err := s.Repos().DNSRecord().FindByIDAndDomain(ctx, recordID, domainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	if !record.IsEditable() {
		return nil, ErrRecordNotEditable
	}

	// Get provider
	dnsProvider, err := providers.NewProvider(providers.DNSProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return nil, err
	}

	dnsProvider.SetDomain(domain.Address)

	// Apply updates
	req.ApplyToModel(record)

	// Update record at provider first
	if err := dnsProvider.UpdateRecord(ctx, toProviderDNSRecord(record)); err != nil {
		return nil, err
	}

	if err := s.Repos().DNSRecord().Update(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

// DeleteRecord deletes a DNS record
func (s *DNSRecordService) DeleteRecord(ctx context.Context, recordID, domainID, teamID string) error {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiberutil.NotFound()
		}
		return err
	}

	record, err := s.Repos().DNSRecord().FindByIDAndDomain(ctx, recordID, domainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiberutil.NotFound()
		}
		return err
	}

	if !record.IsDeletable() {
		return ErrRecordNotDeletable
	}

	// Get provider
	dnsProvider, err := providers.NewProvider(providers.DNSProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return err
	}

	dnsProvider.SetDomain(domain.Address)

	// Delete from provider first, then from database
	if err := dnsProvider.DeleteRecord(ctx, toProviderDNSRecord(record)); err != nil {
		return err
	}

	return s.Repos().DNSRecord().Delete(ctx, record.ID)
}

// GetRecordTypes returns all available record types
func (s *DNSRecordService) GetRecordTypes() []string {
	return GetRecordTypes()
}

// CreateRecordForSite creates a DNS A record for a site
func (s *DNSRecordService) CreateRecordForSite(ctx context.Context, domainID, teamID, siteAddress, serverIP string) error {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiberutil.NotFound()
		}
		return err
	}

	// Determine record name (subdomain)
	recordName := "@"
	baseDomain := domain.Address

	if siteAddress != baseDomain {
		// Extract subdomain (e.g., 'www' from 'www.example.com')
		recordName = strings.TrimSuffix(siteAddress, "."+baseDomain)
	}

	// Create A record request
	comment := "Auto-created for site: " + siteAddress
	req := &dto.CreateDNSRecordRequest{
		Name:    recordName,
		Value:   serverIP,
		Type:    "A",
		TTL:     3600,
		Comment: comment,
	}

	_, err = s.CreateRecord(ctx, domainID, teamID, req)
	return err
}

// GetRecordTypes returns all available record types (package-level function)
func GetRecordTypes() []string {
	types := dnstypes.AllRecordTypes()
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = t.String()
	}

	return result
}

// toProviderDNSRecord converts a models.DNSRecord to providers.DNSRecord
func toProviderDNSRecord(r *models.DNSRecord) *providers.DNSRecord {
	return &providers.DNSRecord{
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
