package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
)

// DnsRecordRepository handles database operations for DNS records
type DnsRecordRepository struct {
	*BaseRepository
}

// NewDnsRecordRepository creates a new DnsRecordRepository instance
func NewDnsRecordRepository(db *gorm.DB) *DnsRecordRepository {
	return &DnsRecordRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new DNS record
func (r *DnsRecordRepository) Create(ctx context.Context, record *models.DnsRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

// FindByID finds a DNS record by ID
func (r *DnsRecordRepository) FindByID(ctx context.Context, id string) (*models.DnsRecord, error) {
	var record models.DnsRecord
	err := r.db.WithContext(ctx).
		First(&record, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// FindByIDAndDomain finds a DNS record by ID and domain
func (r *DnsRecordRepository) FindByIDAndDomain(ctx context.Context, id, domainID string) (*models.DnsRecord, error) {
	var record models.DnsRecord
	err := r.db.WithContext(ctx).
		First(&record, "id = ? AND domain_id = ?", id, domainID).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// FindByDomain finds all DNS records for a domain
func (r *DnsRecordRepository) FindByDomain(ctx context.Context, domainID string) ([]models.DnsRecord, error) {
	var records []models.DnsRecord
	err := r.db.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Order("type ASC, name ASC").
		Find(&records).Error

	return records, err
}

// FindByType finds all DNS records of a specific type for a domain
func (r *DnsRecordRepository) FindByType(ctx context.Context, domainID string, recordType enums.RecordType) ([]models.DnsRecord, error) {
	var records []models.DnsRecord
	err := r.db.WithContext(ctx).
		Where("domain_id = ? AND type = ?", domainID, recordType).
		Order("name ASC").
		Find(&records).Error

	return records, err
}

// Update updates a DNS record
func (r *DnsRecordRepository) Update(ctx context.Context, record *models.DnsRecord) error {
	return r.db.WithContext(ctx).Save(record).Error
}

// UpdateOrCreate updates or creates a DNS record
func (r *DnsRecordRepository) UpdateOrCreate(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*models.DnsRecord, error) {
	var record models.DnsRecord

	// First try to find existing
	err := r.db.WithContext(ctx).Where(where).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		// Create new record with all values
		for k, v := range where {
			update[k] = v
		}
		record = models.DnsRecord{}
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

// Delete deletes a DNS record
func (r *DnsRecordRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.DnsRecord{}, "id = ?", id).Error
}

// DeleteByDomain deletes all DNS records for a domain
func (r *DnsRecordRepository) DeleteByDomain(ctx context.Context, domainID string) error {
	return r.db.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Delete(&models.DnsRecord{}).Error
}
