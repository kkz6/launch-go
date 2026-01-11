package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// PersonalAccessTokenRepository handles personal access token database operations
type PersonalAccessTokenRepository struct {
	db *gorm.DB
}

// NewPersonalAccessTokenRepository creates a new PersonalAccessTokenRepository instance
func NewPersonalAccessTokenRepository(db *gorm.DB) *PersonalAccessTokenRepository {
	return &PersonalAccessTokenRepository{db: db}
}

// Create creates a new personal access token
func (r *PersonalAccessTokenRepository) Create(ctx context.Context, token *models.PersonalAccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// FindByToken finds a personal access token by its token value
func (r *PersonalAccessTokenRepository) FindByToken(ctx context.Context, token string) (*models.PersonalAccessToken, error) {
	var pat models.PersonalAccessToken
	err := r.db.WithContext(ctx).
		Preload("User").
		First(&pat, "token = ?", token).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &pat, nil
}

// UpdateLastUsed updates the last used timestamp
func (r *PersonalAccessTokenRepository) UpdateLastUsed(ctx context.Context, id string) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&models.PersonalAccessToken{}).
		Where("id = ?", id).
		Update("last_used_at", &now).Error
}

// Delete deletes a personal access token
func (r *PersonalAccessTokenRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.PersonalAccessToken{}, "id = ?", id).Error
}

// GetByUser gets all personal access tokens for a user
func (r *PersonalAccessTokenRepository) GetByUser(ctx context.Context, userID string) ([]models.PersonalAccessToken, error) {
	var tokens []models.PersonalAccessToken
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&tokens).Error

	return tokens, err
}
