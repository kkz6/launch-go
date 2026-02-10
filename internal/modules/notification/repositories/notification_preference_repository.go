package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/notification/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// NotificationPreferenceRepository handles database operations for notification preferences
type NotificationPreferenceRepository struct {
	repository.Base[models.NotificationPreference]
}

// NewNotificationPreferenceRepository creates a new notification preference repository
func NewNotificationPreferenceRepository(db *gorm.DB) *NotificationPreferenceRepository {
	return &NotificationPreferenceRepository{
		Base: repository.NewBase[models.NotificationPreference](db),
	}
}

// FindOrCreateByTeamID returns existing preferences or creates with defaults
func (r *NotificationPreferenceRepository) FindOrCreateByTeamID(ctx context.Context, teamID string) (*models.NotificationPreference, error) {
	var pref models.NotificationPreference

	err := r.DB.WithContext(ctx).Where("team_id = ?", teamID).First(&pref).Error
	if err == nil {
		return &pref, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	pref = models.NotificationPreference{
		TeamID:                 teamID,
		EmailServerCreated:     true,
		EmailServerDeleted:     true,
		EmailDeploymentSuccess: false,
		EmailDeploymentFailed:  true,
		EmailBackupSuccess:     false,
		EmailBackupFailed:      true,
	}

	if err := r.DB.WithContext(ctx).Create(&pref).Error; err != nil {
		// Handle race condition: another request may have created it
		var existing models.NotificationPreference
		if findErr := r.DB.WithContext(ctx).Where("team_id = ?", teamID).First(&existing).Error; findErr == nil {
			return &existing, nil
		}

		return nil, err
	}

	return &pref, nil
}

// UpdateByTeamID updates preferences for a team
func (r *NotificationPreferenceRepository) UpdateByTeamID(ctx context.Context, teamID string, pref *models.NotificationPreference) error {
	return r.DB.WithContext(ctx).
		Model(&models.NotificationPreference{}).
		Where("team_id = ?", teamID).
		Updates(map[string]any{
			"email_server_created":     pref.EmailServerCreated,
			"email_server_deleted":     pref.EmailServerDeleted,
			"email_deployment_success": pref.EmailDeploymentSuccess,
			"email_deployment_failed":  pref.EmailDeploymentFailed,
			"email_backup_success":     pref.EmailBackupSuccess,
			"email_backup_failed":      pref.EmailBackupFailed,
		}).Error
}
