package notification

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrChannelNotFound = errors.New("notification channel not found")
)

// Repository handles database operations for notifications
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new notification repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create creates a new notification channel
func (r *Repository) Create(ctx context.Context, channel *NotificationChannel) error {
	return r.db.WithContext(ctx).Create(channel).Error
}

// Update updates an existing notification channel
func (r *Repository) Update(ctx context.Context, channel *NotificationChannel) error {
	return r.db.WithContext(ctx).Save(channel).Error
}

// Delete deletes a notification channel by ID
func (r *Repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&NotificationChannel{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// FindByID finds a notification channel by ID
func (r *Repository) FindByID(ctx context.Context, id string) (*NotificationChannel, error) {
	var channel NotificationChannel
	err := r.db.WithContext(ctx).First(&channel, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	return &channel, nil
}

// FindByIDAndTeamID finds a notification channel by ID and team ID
func (r *Repository) FindByIDAndTeamID(ctx context.Context, id, teamID string) (*NotificationChannel, error) {
	var channel NotificationChannel
	err := r.db.WithContext(ctx).
		Where("id = ? AND team_id = ?", id, teamID).
		First(&channel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	return &channel, nil
}

// FindByTeamID finds all notification channels for a team
func (r *Repository) FindByTeamID(ctx context.Context, teamID string) ([]NotificationChannel, error) {
	var channels []NotificationChannel
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&channels).Error
	return channels, err
}

// FindByUserID finds all notification channels for a user
func (r *Repository) FindByUserID(ctx context.Context, userID string) ([]NotificationChannel, error) {
	var channels []NotificationChannel
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&channels).Error
	return channels, err
}

// FindByProvider finds all notification channels by provider type
func (r *Repository) FindByProvider(ctx context.Context, teamID string, provider ChannelType) ([]NotificationChannel, error) {
	var channels []NotificationChannel
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND provider = ?", teamID, provider).
		Order("created_at DESC").
		Find(&channels).Error
	return channels, err
}

// FindConnected finds all connected notification channels for a team
func (r *Repository) FindConnected(ctx context.Context, teamID string) ([]NotificationChannel, error) {
	var channels []NotificationChannel
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND connected = ?", teamID, true).
		Order("created_at DESC").
		Find(&channels).Error
	return channels, err
}

// SetConnected sets the connected status of a notification channel
func (r *Repository) SetConnected(ctx context.Context, id string, connected bool) error {
	return r.db.WithContext(ctx).
		Model(&NotificationChannel{}).
		Where("id = ?", id).
		Update("connected", connected).Error
}

// SetDefault sets a notification channel as the default for its type
func (r *Repository) SetDefault(ctx context.Context, id string, teamID string, provider ChannelType) error {
	// First, unset any existing default for this provider/team combination
	err := r.db.WithContext(ctx).
		Model(&NotificationChannel{}).
		Where("team_id = ? AND provider = ? AND is_default = ?", teamID, provider, true).
		Update("is_default", false).Error
	if err != nil {
		return err
	}

	// Set the new default
	return r.db.WithContext(ctx).
		Model(&NotificationChannel{}).
		Where("id = ?", id).
		Update("is_default", true).Error
}

// Exists checks if a notification channel exists
func (r *Repository) Exists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&NotificationChannel{}).
		Where("id = ?", id).
		Count(&count).Error
	return count > 0, err
}

// CountByTeamID counts the notification channels for a team
func (r *Repository) CountByTeamID(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&NotificationChannel{}).
		Where("team_id = ?", teamID).
		Count(&count).Error
	return count, err
}

// UpdateData updates only the data field of a notification channel
func (r *Repository) UpdateData(ctx context.Context, id string, data ChannelData) error {
	return r.db.WithContext(ctx).
		Model(&NotificationChannel{}).
		Where("id = ?", id).
		Update("data", data).Error
}
