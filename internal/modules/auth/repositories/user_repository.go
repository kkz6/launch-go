package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// UserRepository handles user database operations
type UserRepository struct {
	repository.Base[models.User]
}

// NewUserRepository creates a new UserRepository instance
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		Base: repository.NewBase[models.User](db),
	}
}

// FindByID finds a user by their ID with preloaded relations
func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	err := r.DB.WithContext(ctx).
		Preload("CurrentTeam").
		Preload("Teams").
		First(&user, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

// FindByEmail finds a user by their email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.DB.WithContext(ctx).
		Preload("CurrentTeam").
		Preload("Teams").
		First(&user, "email = ?", email).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

// ExistsByEmail checks if a user with the given email exists
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.User{}).
		Where("email = ?", email).
		Count(&count).Error

	return count > 0, err
}

// SetCurrentTeam sets the user's current team
func (r *UserRepository) SetCurrentTeam(ctx context.Context, userID, teamID string) error {
	return r.Base.UpdateFields(ctx, userID, map[string]interface{}{
		"current_team_id": teamID,
	})
}

// MarkEmailAsVerified marks a user's email as verified
func (r *UserRepository) MarkEmailAsVerified(ctx context.Context, userID string) error {
	now := time.Now()
	return r.Base.UpdateFields(ctx, userID, map[string]interface{}{
		"email_verified_at": &now,
	})
}

// SetOnboarded sets the user's onboarded status
func (r *UserRepository) SetOnboarded(ctx context.Context, userID string, onboarded bool) error {
	return r.Base.UpdateFields(ctx, userID, map[string]interface{}{
		"onboarded": onboarded,
	})
}
