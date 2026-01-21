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

// Create creates a new personal access token
func (r *PersonalAccessTokenRepository) Create(ctx context.Context, token *models.PersonalAccessToken) error {
	return r.Base.Create(ctx, token)
}

// FindByToken finds a personal access token by its token value
func (r *PersonalAccessTokenRepository) FindByToken(ctx context.Context, token string) (*models.PersonalAccessToken, error) {
	var pat models.PersonalAccessToken
	err := r.DB.WithContext(ctx).
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
	return r.Base.UpdateFields(ctx, id, map[string]interface{}{
		"last_used_at": &now,
	})
}

// Delete deletes a personal access token
func (r *PersonalAccessTokenRepository) Delete(ctx context.Context, id string) error {
	return r.Base.Delete(ctx, id)
}

// GetByUser gets all personal access tokens for a user
func (r *PersonalAccessTokenRepository) GetByUser(ctx context.Context, userID string) ([]models.PersonalAccessToken, error) {
	return r.Base.FindByUser(ctx, userID)
}
