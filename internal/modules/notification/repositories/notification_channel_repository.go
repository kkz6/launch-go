package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// NotificationChannelRepository handles database operations for notification channels
type NotificationChannelRepository struct {
	repository.Base[models.NotificationChannel]
}

// NewNotificationChannelRepository creates a new notification channel repository
func NewNotificationChannelRepository(db *gorm.DB) *NotificationChannelRepository {
	return &NotificationChannelRepository{
		Base: repository.NewBase[models.NotificationChannel](db),
	}
}

// Create creates a new notification channel
func (r *NotificationChannelRepository) Create(ctx context.Context, channel *models.NotificationChannel) error {
	return r.DB.WithContext(ctx).Create(channel).Error
}

// Update updates an existing notification channel
func (r *NotificationChannelRepository) Update(ctx context.Context, channel *models.NotificationChannel) error {
	return r.DB.WithContext(ctx).Save(channel).Error
}

// Delete deletes a notification channel by ID
func (r *NotificationChannelRepository) Delete(ctx context.Context, id string) error {
	result := r.DB.WithContext(ctx).Delete(&models.NotificationChannel{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fiberutil.NotFound()
	}

	return nil
}

// FindByID finds a notification channel by ID
func (r *NotificationChannelRepository) FindByID(ctx context.Context, id string) (*models.NotificationChannel, error) {
	var channel models.NotificationChannel

	err := r.DB.WithContext(ctx).First(&channel, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}

		return nil, err
	}

	return &channel, nil
}

// FindByIDAndTeamID finds a notification channel by ID and team ID
func (r *NotificationChannelRepository) FindByIDAndTeamID(ctx context.Context, id, teamID string) (*models.NotificationChannel, error) {
	var channel models.NotificationChannel

	err := r.DB.WithContext(ctx).
		Where("id = ? AND team_id = ?", id, teamID).
		First(&channel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}

		return nil, err
	}

	return &channel, nil
}

// FindByTeamID finds all notification channels for a team
func (r *NotificationChannelRepository) FindByTeamID(ctx context.Context, teamID string) ([]models.NotificationChannel, error) {
	var channels []models.NotificationChannel

	err := r.DB.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&channels).Error

	return channels, err
}

// FindByUserID finds all notification channels for a user
func (r *NotificationChannelRepository) FindByUserID(ctx context.Context, userID string) ([]models.NotificationChannel, error) {
	var channels []models.NotificationChannel

	err := r.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&channels).Error

	return channels, err
}

// FindByProvider finds all notification channels by provider type
func (r *NotificationChannelRepository) FindByProvider(ctx context.Context, teamID string, provider enums.ChannelType) ([]models.NotificationChannel, error) {
	var channels []models.NotificationChannel

	err := r.DB.WithContext(ctx).
		Where("team_id = ? AND provider = ?", teamID, provider).
		Order("created_at DESC").
		Find(&channels).Error

	return channels, err
}

// FindConnected finds all connected notification channels for a team
func (r *NotificationChannelRepository) FindConnected(ctx context.Context, teamID string) ([]models.NotificationChannel, error) {
	var channels []models.NotificationChannel

	err := r.DB.WithContext(ctx).
		Where("team_id = ? AND connected = ?", teamID, true).
		Order("created_at DESC").
		Find(&channels).Error

	return channels, err
}

// SetConnected sets the connected status of a notification channel
func (r *NotificationChannelRepository) SetConnected(ctx context.Context, id string, connected bool) error {
	return r.DB.WithContext(ctx).
		Model(&models.NotificationChannel{}).
		Where("id = ?", id).
		Update("connected", connected).Error
}

// SetDefault sets a notification channel as the default for its type
func (r *NotificationChannelRepository) SetDefault(ctx context.Context, id string, teamID string, provider enums.ChannelType) error {
	// First, unset any existing default for this provider/team combination
	err := r.DB.WithContext(ctx).
		Model(&models.NotificationChannel{}).
		Where("team_id = ? AND provider = ? AND is_default = ?", teamID, provider, true).
		Update("is_default", false).Error
	if err != nil {
		return err
	}

	// Set the new default
	return r.DB.WithContext(ctx).
		Model(&models.NotificationChannel{}).
		Where("id = ?", id).
		Update("is_default", true).Error
}

// Exists checks if a notification channel exists
func (r *NotificationChannelRepository) Exists(ctx context.Context, id string) (bool, error) {
	var count int64

	err := r.DB.WithContext(ctx).
		Model(&models.NotificationChannel{}).
		Where("id = ?", id).
		Count(&count).Error

	return count > 0, err
}

// CountByTeamID counts the notification channels for a team
func (r *NotificationChannelRepository) CountByTeamID(ctx context.Context, teamID string) (int64, error) {
	var count int64

	err := r.DB.WithContext(ctx).
		Model(&models.NotificationChannel{}).
		Where("team_id = ?", teamID).
		Count(&count).Error

	return count, err
}

// UpdateData updates only the data field of a notification channel
func (r *NotificationChannelRepository) UpdateData(ctx context.Context, id string, data models.ChannelData) error {
	return r.DB.WithContext(ctx).
		Model(&models.NotificationChannel{}).
		Where("id = ?", id).
		Update("data", data).Error
}
