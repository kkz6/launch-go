package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// CertificateRepository handles database operations for certificates.
// Embeds repository.Base[T] for common CRUD operations.
type CertificateRepository struct {
	repository.Base[models.Certificate]
}

// NewCertificateRepository creates a new certificate repository
func NewCertificateRepository(db *gorm.DB) *CertificateRepository {
	return &CertificateRepository{
		Base: repository.NewBase[models.Certificate](db),
	}
}

// FindByID finds a certificate by ID with custom error.
// Wraps the generic FindByID to return ErrCertificateNotFound.
func (r *CertificateRepository) FindByID(ctx context.Context, id string) (*models.Certificate, error) {
	cert, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrCertificateNotFound
		}
		return nil, err
	}
	return cert, nil
}

// FindBySite finds all certificates for a site
func (r *CertificateRepository) FindBySite(ctx context.Context, siteID string) ([]models.Certificate, error) {
	var certs []models.Certificate
	err := r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&certs).Error

	return certs, err
}

// FindActiveBySite finds the active certificate for a site
func (r *CertificateRepository) FindActiveBySite(ctx context.Context, siteID string) (*models.Certificate, error) {
	var cert models.Certificate
	err := r.DB.WithContext(ctx).
		Where("site_id = ? AND is_active = ?", siteID, true).
		First(&cert).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &cert, nil
}

// DeactivateAll deactivates all certificates for a site
func (r *CertificateRepository) DeactivateAll(ctx context.Context, siteID string) error {
	return r.DB.WithContext(ctx).
		Model(&models.Certificate{}).
		Where("site_id = ?", siteID).
		Update("is_active", false).Error
}

// DeleteBySite deletes all certificates for a site
func (r *CertificateRepository) DeleteBySite(ctx context.Context, siteID string) error {
	return r.DB.WithContext(ctx).
		Where("site_id = ?", siteID).
		Delete(&models.Certificate{}).Error
}

// Note: The following methods are inherited from repository.Base[T]:
// - Create(ctx, entity) error
// - Update(ctx, entity) error
// - Delete(ctx, id) error
// - UpdateFields(ctx, id, fields) error
// - Exists(ctx, id) (bool, error)
// - Count(ctx) (int64, error)
// - Transaction(ctx, fn) error
// - Query(ctx) *gorm.DB
// - WithPreload(ctx, relations...) *gorm.DB
