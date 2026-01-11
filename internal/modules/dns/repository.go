package dns

import (
	"context"

	"gorm.io/gorm"
)

// Repository handles database operations for the DNS module
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// DomainProvider Methods

// CreateDomainProvider creates a new domain provider
func (r *Repository) CreateDomainProvider(ctx context.Context, provider *DomainProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// FindDomainProviderByID finds a domain provider by ID
func (r *Repository) FindDomainProviderByID(ctx context.Context, id string) (*DomainProvider, error) {
	var provider DomainProvider
	err := r.db.WithContext(ctx).
		Preload("Domains").
		First(&provider, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// FindDomainProviderByIDAndTeam finds a domain provider by ID and team
func (r *Repository) FindDomainProviderByIDAndTeam(ctx context.Context, id, teamID string) (*DomainProvider, error) {
	var provider DomainProvider
	err := r.db.WithContext(ctx).
		Preload("Domains").
		First(&provider, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// FindDomainProvidersByTeam finds all domain providers for a team
func (r *Repository) FindDomainProvidersByTeam(ctx context.Context, teamID string) ([]DomainProvider, error) {
	var providers []DomainProvider
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error
	return providers, err
}

// FindDomainProvidersByTeamWithDomainCount finds all domain providers for a team with domain count
func (r *Repository) FindDomainProvidersByTeamWithDomainCount(ctx context.Context, teamID string) ([]DomainProvider, map[string]int, error) {
	var providers []DomainProvider
	err := r.db.WithContext(ctx).
		Preload("Domains").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error
	if err != nil {
		return nil, nil, err
	}

	counts := make(map[string]int)
	for _, p := range providers {
		counts[p.ID] = len(p.Domains)
	}

	return providers, counts, nil
}

// UpdateDomainProvider updates a domain provider
func (r *Repository) UpdateDomainProvider(ctx context.Context, provider *DomainProvider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// UpdateDomainProviderFields updates specific fields of a domain provider
func (r *Repository) UpdateDomainProviderFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&DomainProvider{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// DeleteDomainProvider deletes a domain provider
func (r *Repository) DeleteDomainProvider(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&DomainProvider{}, "id = ?", id).Error
}

// CountDomainsByProvider counts domains for a provider
func (r *Repository) CountDomainsByProvider(ctx context.Context, providerID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Domain{}).
		Where("domain_provider_id = ?", providerID).
		Count(&count).Error
	return count, err
}

// Domain Methods

// CreateDomain creates a new domain
func (r *Repository) CreateDomain(ctx context.Context, domain *Domain) error {
	return r.db.WithContext(ctx).Create(domain).Error
}

// FindDomainByID finds a domain by ID
func (r *Repository) FindDomainByID(ctx context.Context, id string) (*Domain, error) {
	var domain Domain
	err := r.db.WithContext(ctx).
		Preload("Provider").
		Preload("Records").
		First(&domain, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

// FindDomainByIDAndTeam finds a domain by ID and team
func (r *Repository) FindDomainByIDAndTeam(ctx context.Context, id, teamID string) (*Domain, error) {
	var domain Domain
	err := r.db.WithContext(ctx).
		Preload("Provider").
		Preload("Records", func(db *gorm.DB) *gorm.DB {
			return db.Order("type ASC, name ASC")
		}).
		First(&domain, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

// FindDomainsByTeam finds all domains for a team
func (r *Repository) FindDomainsByTeam(ctx context.Context, teamID string) ([]Domain, error) {
	var domains []Domain
	err := r.db.WithContext(ctx).
		Preload("Provider").
		Preload("Records").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&domains).Error
	return domains, err
}

// FindDomainsByProvider finds all domains for a provider
func (r *Repository) FindDomainsByProvider(ctx context.Context, providerID string) ([]Domain, error) {
	var domains []Domain
	err := r.db.WithContext(ctx).
		Preload("Records").
		Where("domain_provider_id = ?", providerID).
		Find(&domains).Error
	return domains, err
}

// FindDomainByAddressAndProvider finds a domain by address and provider
func (r *Repository) FindDomainByAddressAndProvider(ctx context.Context, address, providerID string) (*Domain, error) {
	var domain Domain
	err := r.db.WithContext(ctx).
		Where("address = ? AND domain_provider_id = ?", address, providerID).
		First(&domain).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

// UpdateDomain updates a domain
func (r *Repository) UpdateDomain(ctx context.Context, domain *Domain) error {
	return r.db.WithContext(ctx).Save(domain).Error
}

// UpdateOrCreateDomain updates or creates a domain
func (r *Repository) UpdateOrCreateDomain(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*Domain, error) {
	var domain Domain

	// First try to find existing
	err := r.db.WithContext(ctx).Where(where).First(&domain).Error
	if err == gorm.ErrRecordNotFound {
		// Create new domain with all values
		for k, v := range where {
			update[k] = v
		}
		domain = Domain{}
		if err := r.db.WithContext(ctx).Model(&domain).Create(update).Error; err != nil {
			return nil, err
		}
		// Reload the domain
		return r.FindDomainByAddressAndProvider(ctx, where["address"].(string), where["domain_provider_id"].(string))
	}

	if err != nil {
		return nil, err
	}

	// Update existing
	if err := r.db.WithContext(ctx).Model(&domain).Updates(update).Error; err != nil {
		return nil, err
	}

	return &domain, nil
}

// DeleteDomain deletes a domain
func (r *Repository) DeleteDomain(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Domain{}, "id = ?", id).Error
}

// DnsRecord Methods

// CreateDnsRecord creates a new DNS record
func (r *Repository) CreateDnsRecord(ctx context.Context, record *DnsRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

// FindDnsRecordByID finds a DNS record by ID
func (r *Repository) FindDnsRecordByID(ctx context.Context, id string) (*DnsRecord, error) {
	var record DnsRecord
	err := r.db.WithContext(ctx).
		First(&record, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// FindDnsRecordByIDAndDomain finds a DNS record by ID and domain
func (r *Repository) FindDnsRecordByIDAndDomain(ctx context.Context, id, domainID string) (*DnsRecord, error) {
	var record DnsRecord
	err := r.db.WithContext(ctx).
		First(&record, "id = ? AND domain_id = ?", id, domainID).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// FindDnsRecordsByDomain finds all DNS records for a domain
func (r *Repository) FindDnsRecordsByDomain(ctx context.Context, domainID string) ([]DnsRecord, error) {
	var records []DnsRecord
	err := r.db.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Order("type ASC, name ASC").
		Find(&records).Error
	return records, err
}

// FindDnsRecordsByType finds all DNS records of a specific type for a domain
func (r *Repository) FindDnsRecordsByType(ctx context.Context, domainID string, recordType RecordType) ([]DnsRecord, error) {
	var records []DnsRecord
	err := r.db.WithContext(ctx).
		Where("domain_id = ? AND type = ?", domainID, recordType).
		Order("name ASC").
		Find(&records).Error
	return records, err
}

// UpdateDnsRecord updates a DNS record
func (r *Repository) UpdateDnsRecord(ctx context.Context, record *DnsRecord) error {
	return r.db.WithContext(ctx).Save(record).Error
}

// UpdateOrCreateDnsRecord updates or creates a DNS record
func (r *Repository) UpdateOrCreateDnsRecord(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*DnsRecord, error) {
	var record DnsRecord

	// First try to find existing
	err := r.db.WithContext(ctx).Where(where).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		// Create new record with all values
		for k, v := range where {
			update[k] = v
		}
		record = DnsRecord{}
		if err := r.db.WithContext(ctx).Model(&record).Create(update).Error; err != nil {
			return nil, err
		}
		return &record, nil
	}

	if err != nil {
		return nil, err
	}

	// Update existing
	if err := r.db.WithContext(ctx).Model(&record).Updates(update).Error; err != nil {
		return nil, err
	}

	return &record, nil
}

// DeleteDnsRecord deletes a DNS record
func (r *Repository) DeleteDnsRecord(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&DnsRecord{}, "id = ?", id).Error
}

// DeleteDnsRecordsByDomain deletes all DNS records for a domain
func (r *Repository) DeleteDnsRecordsByDomain(ctx context.Context, domainID string) error {
	return r.db.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Delete(&DnsRecord{}).Error
}

// Transaction Methods

// BeginTransaction begins a new transaction
func (r *Repository) BeginTransaction(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Begin()
}

// WithTransaction executes a function within a transaction
func (r *Repository) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}
