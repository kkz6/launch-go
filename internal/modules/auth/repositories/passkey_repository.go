package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// PasskeyRepository handles passkey database operations
type PasskeyRepository struct {
	repository.Base[models.Passkey]
}

// NewPasskeyRepository creates a new PasskeyRepository instance
func NewPasskeyRepository(db *gorm.DB) *PasskeyRepository {
	return &PasskeyRepository{
		Base: repository.NewBase[models.Passkey](db),
	}
}

// FindByUserID finds all passkeys for a user
func (r *PasskeyRepository) FindByUserID(ctx context.Context, userID string) ([]models.Passkey, error) {
	return r.Base.FindByUser(ctx, userID)
}

// FindByID finds a passkey by ID
func (r *PasskeyRepository) FindByID(ctx context.Context, id string) (*models.Passkey, error) {
	return r.Base.FindByID(ctx, id)
}

// FindByCredentialID finds a passkey by credential ID
func (r *PasskeyRepository) FindByCredentialID(ctx context.Context, credentialID string) (*models.Passkey, error) {
	var passkey models.Passkey
	err := r.DB.WithContext(ctx).First(&passkey, "credential_id = ?", credentialID).Error
	if err != nil {
		return nil, err
	}

	return &passkey, nil
}

// Create creates a new passkey
func (r *PasskeyRepository) Create(ctx context.Context, passkey *models.Passkey) error {
	return r.Base.Create(ctx, passkey)
}

// Update updates an existing passkey
func (r *PasskeyRepository) Update(ctx context.Context, passkey *models.Passkey) error {
	return r.Base.Update(ctx, passkey)
}

// Delete deletes a passkey by ID
func (r *PasskeyRepository) Delete(ctx context.Context, id string) error {
	return r.Base.Delete(ctx, id)
}

// DeleteByUserID deletes a passkey by ID and user ID (ensures user owns the passkey)
func (r *PasskeyRepository) DeleteByUserID(ctx context.Context, id, userID string) error {
	result := r.DB.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.Passkey{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// CountByUserID counts passkeys for a user
func (r *PasskeyRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Passkey{}).
		Where("user_id = ?", userID).
		Count(&count).Error

	return count, err
}
