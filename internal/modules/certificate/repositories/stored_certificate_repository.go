package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/certificate/models"
)

type StoredCertificateRepository struct {
	db *gorm.DB
}

func NewStoredCertificateRepository(db *gorm.DB) *StoredCertificateRepository {
	return &StoredCertificateRepository{db: db}
}

func (r *StoredCertificateRepository) Create(ctx context.Context, c *models.StoredCertificate) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *StoredCertificateRepository) FindByID(ctx context.Context, teamID, id string) (*models.StoredCertificate, error) {
	var c models.StoredCertificate
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND id = ?", teamID, id).
		First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *StoredCertificateRepository) List(ctx context.Context, teamID string) ([]models.StoredCertificate, error) {
	var out []models.StoredCertificate
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("not_after ASC").
		Find(&out).Error
	return out, err
}

func (r *StoredCertificateRepository) Update(ctx context.Context, c *models.StoredCertificate) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *StoredCertificateRepository) SoftDelete(ctx context.Context, teamID, id string) error {
	return r.db.WithContext(ctx).
		Where("team_id = ? AND id = ?", teamID, id).
		Delete(&models.StoredCertificate{}).Error
}

func (r *StoredCertificateRepository) FindByFingerprint(ctx context.Context, teamID, fingerprint string) (*models.StoredCertificate, error) {
	var c models.StoredCertificate
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND fingerprint_sha256 = ?", teamID, fingerprint).
		First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}
