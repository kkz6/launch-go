package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/platform/models"
	"github.com/kkz6/launch-go/internal/pkg/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DismissalRepository handles user dismissals for platform update banners
type DismissalRepository struct {
	db *gorm.DB
}

// NewDismissalRepository creates a new DismissalRepository
func NewDismissalRepository(db *gorm.DB) *DismissalRepository {
	return &DismissalRepository{db: db}
}

// Dismiss creates a dismissal record for a user and platform update
func (r *DismissalRepository) Dismiss(ctx context.Context, userID, platformUpdateID string) error {
	dismissal := &models.PlatformUpdateDismissal{
		ID:               util.NewULID(),
		UserID:           userID,
		PlatformUpdateID: platformUpdateID,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "platform_update_id"}},
			DoNothing: true,
		}).
		Create(dismissal).Error
}

// IsDismissed checks if a user has dismissed a specific update
func (r *DismissalRepository) IsDismissed(ctx context.Context, userID, platformUpdateID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.PlatformUpdateDismissal{}).
		Where("user_id = ? AND platform_update_id = ?", userID, platformUpdateID).
		Count(&count).Error

	return count > 0, err
}
