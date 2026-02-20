package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// PersonalAccessTokenRepository handles personal access token database operations
type PersonalAccessTokenRepository struct {
	repository.Base[models.PersonalAccessToken]
}

// NewPersonalAccessTokenRepository creates a new PersonalAccessTokenRepository instance
func NewPersonalAccessTokenRepository(db *gorm.DB) *PersonalAccessTokenRepository {
	return &PersonalAccessTokenRepository{
		Base: repository.NewBase[models.PersonalAccessToken](db),
	}
}

// FindByToken finds a personal access token by its token value
func (r *PersonalAccessTokenRepository) FindByToken(ctx context.Context, token string) (*models.PersonalAccessToken, error) {
	var pat models.PersonalAccessToken
	err := r.DB.WithContext(ctx).
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
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"last_used_at": &now,
	})
}

// GetByUser gets all personal access tokens for a user (uses polymorphic tokenable_id)
func (r *PersonalAccessTokenRepository) GetByUser(ctx context.Context, userID string) ([]models.PersonalAccessToken, error) {
	var tokens []models.PersonalAccessToken
	err := r.DB.WithContext(ctx).
		Where("tokenable_type = ? AND tokenable_id = ?", "User", userID).
		Order("created_at DESC").
		Find(&tokens).Error

	return tokens, err
}

// DeleteByUser deletes a token owned by a specific user
func (r *PersonalAccessTokenRepository) DeleteByUser(ctx context.Context, id, userID string) (int64, error) {
	result := r.DB.WithContext(ctx).
		Where("id = ? AND tokenable_type = ? AND tokenable_id = ?", id, "User", userID).
		Delete(&models.PersonalAccessToken{})

	return result.RowsAffected, result.Error
}
