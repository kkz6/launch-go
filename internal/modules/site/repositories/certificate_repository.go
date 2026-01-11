package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// CertificateRepository handles database operations for certificates
type CertificateRepository struct {
	*BaseRepository
}

// NewCertificateRepository creates a new certificate repository
func NewCertificateRepository(db *gorm.DB) *CertificateRepository {
	return &CertificateRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new certificate
func (r *CertificateRepository) Create(ctx context.Context, cert *models.Certificate) error {
	return r.db.WithContext(ctx).Create(cert).Error
}

// FindByID finds a certificate by ID
func (r *CertificateRepository) FindByID(ctx context.Context, id string) (*models.Certificate, error) {
	var cert models.Certificate
	err := r.db.WithContext(ctx).First(&cert, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCertificateNotFound
		}

		return nil, err
	}

	return &cert, nil
}

// FindBySite finds all certificates for a site
func (r *CertificateRepository) FindBySite(ctx context.Context, siteID string) ([]models.Certificate, error) {
	var certs []models.Certificate
	err := r.db.WithContext(ctx).
		Where("site_id = ?", siteID).
		Order("created_at DESC").
		Find(&certs).Error

	return certs, err
}

// FindActiveBySite finds the active certificate for a site
func (r *CertificateRepository) FindActiveBySite(ctx context.Context, siteID string) (*models.Certificate, error) {
	var cert models.Certificate
	err := r.db.WithContext(ctx).
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

// Update updates a certificate
func (r *CertificateRepository) Update(ctx context.Context, cert *models.Certificate) error {
	return r.db.WithContext(ctx).Save(cert).Error
}

// Delete deletes a certificate
func (r *CertificateRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Certificate{}, "id = ?", id).Error
}

// DeactivateAll deactivates all certificates for a site
func (r *CertificateRepository) DeactivateAll(ctx context.Context, siteID string) error {
	return r.db.WithContext(ctx).
		Model(&models.Certificate{}).
		Where("site_id = ?", siteID).
		Update("is_active", false).Error
}
