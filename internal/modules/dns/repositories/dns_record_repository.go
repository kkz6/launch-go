package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DNSRecordRepository handles database operations for DNS records.
// Generic CRUD comes from repository.Base[DNSRecord].
type DNSRecordRepository struct {
	repository.Base[models.DNSRecord]
}

// NewDNSRecordRepository creates a new DNSRecordRepository instance.
func NewDNSRecordRepository(db *gorm.DB) *DNSRecordRepository {
	return &DNSRecordRepository{
		Base: repository.NewBase[models.DNSRecord](db),
	}
}

// FindByIDAndDomain finds a DNS record by ID scoped to a domain.
func (r *DNSRecordRepository) FindByIDAndDomain(ctx context.Context, id, domainID string) (*models.DNSRecord, error) {
	var record models.DNSRecord
	err := r.DB.WithContext(ctx).
		First(&record, "id = ? AND domain_id = ?", id, domainID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &record, nil
}

// FindByDomain finds all DNS records for a domain ordered by type, name.
func (r *DNSRecordRepository) FindByDomain(ctx context.Context, domainID string) ([]models.DNSRecord, error) {
	var records []models.DNSRecord
	err := r.DB.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Order("type ASC, name ASC").
		Find(&records).Error
	return records, err
}

// UpdateOrCreate updates an existing record matching `where`, or creates one.
func (r *DNSRecordRepository) UpdateOrCreate(ctx context.Context, where map[string]interface{}, update map[string]interface{}) (*models.DNSRecord, error) {
	var record models.DNSRecord
	err := r.DB.WithContext(ctx).
		Where(where).
		Assign(update).
		FirstOrCreate(&record).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}
