package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DNSRecordRepository handles database operations for DNS records
type DNSRecordRepository struct {
	repository.Base[models.DNSRecord]
}

// NewDNSRecordRepository creates a new DNSRecordRepository instance
func NewDNSRecordRepository(db *gorm.DB) *DNSRecordRepository {
	return &DNSRecordRepository{
		Base: repository.NewBase[models.DNSRecord](db),
	}
}

// Create creates a new DNS record
func (r *DNSRecordRepository) Create(ctx context.Context, record *models.DNSRecord) error {
	return r.DB.WithContext(ctx).Create(record).Error
}

// FindByID finds a DNS record by ID
func (r *DNSRecordRepository) FindByID(ctx context.Context, id string) (*models.DNSRecord, error) {
	var record models.DNSRecord
	err := r.DB.WithContext(ctx).
		First(&record, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// FindByIDAndDomain finds a DNS record by ID and domain
func (r *DNSRecordRepository) FindByIDAndDomain(ctx context.Context, id, domainID string) (*models.DNSRecord, error) {
	var record models.DNSRecord
	err := r.DB.WithContext(ctx).
		First(&record, "id = ? AND domain_id = ?", id, domainID).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// FindByDomain finds all DNS records for a domain
func (r *DNSRecordRepository) FindByDomain(ctx context.Context, domainID string) ([]models.DNSRecord, error) {
	var records []models.DNSRecord
	err := r.DB.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Order("type ASC, name ASC").
		Find(&records).Error

	return records, err
}

// FindByType finds all DNS records of a specific type for a domain
func (r *DNSRecordRepository) FindByType(ctx context.Context, domainID string, recordType enums.RecordType) ([]models.DNSRecord, error) {
	var records []models.DNSRecord
	err := r.DB.WithContext(ctx).
		Where("domain_id = ? AND type = ?", domainID, recordType).
		Order("name ASC").
		Find(&records).Error

	return records, err
}

// Update updates a DNS record
func (r *DNSRecordRepository) Update(ctx context.Context, record *models.DNSRecord) error {
	return r.DB.WithContext(ctx).Save(record).Error
}

// UpdateOrCreate updates or creates a DNS record
func (r *DNSRecordRepository) UpdateOrCreate(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*models.DNSRecord, error) {
	var record models.DNSRecord

	// First try to find existing
	err := r.DB.WithContext(ctx).Where(where).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		// Create new record with all values
		for k, v := range where {
			update[k] = v
		}
		record = models.DNSRecord{}
		if err := r.DB.WithContext(ctx).Model(&record).Create(update).Error; err != nil {
			return nil, err
		}

		return &record, nil
	}

	if err != nil {
		return nil, err
	}

	// Update existing
	if err := r.DB.WithContext(ctx).Model(&record).Updates(update).Error; err != nil {
		return nil, err
	}

	return &record, nil
}

// Delete deletes a DNS record
func (r *DNSRecordRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.DNSRecord{}, "id = ?", id).Error
}

// DeleteByDomain deletes all DNS records for a domain
func (r *DNSRecordRepository) DeleteByDomain(ctx context.Context, domainID string) error {
	return r.DB.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Delete(&models.DNSRecord{}).Error
}
