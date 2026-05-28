package services

import (
	"context"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	dnstypes "github.com/kkz6/launch-go/internal/modules/dns/types"
)

// DNSRecordService handles business logic for DNS records.
type DNSRecordService struct {
	*BaseService
}

// NewDNSRecordService creates a new DNSRecordService instance.
func NewDNSRecordService(deps *ServiceDeps) *DNSRecordService {
	return &DNSRecordService{BaseService: NewBaseService(deps)}
}

// CreateRecord creates a new DNS record under a domain and returns the
// response DTO. userID is part of the framework-mutation convention.
func (s *DNSRecordService) CreateRecord(ctx context.Context, domainID, teamID, userID string, req *dto.CreateDNSRecordRequest) (dto.DNSRecordResponse, error) {
	_ = userID
	record, err := s.buildAndDispatchRecord(ctx, domainID, teamID, req)
	if err != nil {
		return dto.DNSRecordResponse{}, err
	}
	return dto.ToDNSRecordResponse(record), nil
}

// createRecord runs the create flow and returns the created model.
func (s *DNSRecordService) buildAndDispatchRecord(ctx context.Context, domainID, teamID string, req *dto.CreateDNSRecordRequest) (*models.DNSRecord, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		return nil, notFoundAs(err, "Domain not found")
	}

	record := req.ToModel(domainID)
	record.TeamID = domain.TeamID

	dnsProvider, err := providers.NewProvider(providers.DNSProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return nil, err
	}
	dnsProvider.SetDomain(domain.Address)

	providerID, err := dnsProvider.AddRecord(ctx, toProviderDNSRecord(record))
	if err != nil {
		return nil, wrapProviderErr(err)
	}
	record.ProviderID = providerID

	if err := s.Repos().DNSRecord().Create(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

// UpdateRecord updates a DNS record under a domain and returns the
// response DTO. userID is part of the framework-mutation convention.
func (s *DNSRecordService) UpdateRecord(ctx context.Context, recordID, domainID, teamID, userID string, req *dto.UpdateDNSRecordRequest) (dto.DNSRecordResponse, error) {
	_ = userID
	record, err := s.applyRecordUpdate(ctx, recordID, domainID, teamID, req)
	if err != nil {
		return dto.DNSRecordResponse{}, err
	}
	return dto.ToDNSRecordResponse(record), nil
}

// updateRecord runs the update flow and returns the updated model.
func (s *DNSRecordService) applyRecordUpdate(ctx context.Context, recordID, domainID, teamID string, req *dto.UpdateDNSRecordRequest) (*models.DNSRecord, error) {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		return nil, notFoundAs(err, "Record or domain not found")
	}

	record, err := s.Repos().DNSRecord().FindByIDAndDomain(ctx, recordID, domainID)
	if err != nil {
		return nil, notFoundAs(err, "Record or domain not found")
	}

	if !record.IsEditable() {
		return nil, ErrRecordNotEditable
	}

	dnsProvider, err := providers.NewProvider(providers.DNSProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return nil, err
	}
	dnsProvider.SetDomain(domain.Address)

	req.ApplyToModel(record)

	if err := dnsProvider.UpdateRecord(ctx, toProviderDNSRecord(record)); err != nil {
		return nil, wrapProviderErr(err)
	}

	if err := s.Repos().DNSRecord().Update(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

// DeleteRecord deletes a DNS record from the provider and the database.
// userID is part of the framework-mutation convention.
func (s *DNSRecordService) DeleteRecord(ctx context.Context, recordID, domainID, teamID, userID string) error {
	_ = userID
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		return notFoundAs(err, "Record or domain not found")
	}

	record, err := s.Repos().DNSRecord().FindByIDAndDomain(ctx, recordID, domainID)
	if err != nil {
		return notFoundAs(err, "Record or domain not found")
	}

	if !record.IsDeletable() {
		return ErrRecordNotDeletable
	}

	dnsProvider, err := providers.NewProvider(providers.DNSProviderType(domain.Provider.Provider), domain.Provider.Credentials, domain.Provider.AdditionalData)
	if err != nil {
		return err
	}
	dnsProvider.SetDomain(domain.Address)

	if err := dnsProvider.DeleteRecord(ctx, toProviderDNSRecord(record)); err != nil {
		return wrapProviderErr(err)
	}

	return s.Repos().DNSRecord().Delete(ctx, record.ID)
}

// GetRecordTypes returns all available record types.
func (s *DNSRecordService) GetRecordTypes() []string {
	return GetRecordTypes()
}

// CreateRecordForSite creates a DNS A record for a site.
func (s *DNSRecordService) CreateRecordForSite(ctx context.Context, domainID, teamID, siteAddress, serverIP string) error {
	domain, err := s.Repos().Domain().FindByIDAndTeam(ctx, domainID, teamID)
	if err != nil {
		return notFoundAs(err, "Domain not found")
	}

	recordName := "@"
	if siteAddress != domain.Address {
		recordName = strings.TrimSuffix(siteAddress, "."+domain.Address)
	}

	req := &dto.CreateDNSRecordRequest{
		Name:    recordName,
		Value:   serverIP,
		Type:    "A",
		TTL:     3600,
		Comment: "Auto-created for site: " + siteAddress,
	}
	_, err = s.CreateRecord(ctx, domainID, teamID, "", req)
	return err
}

// GetRecordTypes returns all available record types (package-level function).
func GetRecordTypes() []string {
	types := dnstypes.AllRecordTypes()
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = t.String()
	}
	return result
}

// toProviderDNSRecord converts a models.DNSRecord to providers.DNSRecord.
func toProviderDNSRecord(r *models.DNSRecord) *providers.DNSRecord {
	return &providers.DNSRecord{
		ID:         r.ID,
		ProviderID: r.ProviderID,
		Type:       r.Type,
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
