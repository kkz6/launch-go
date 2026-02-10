package repositories

import (
	"gorm.io/gorm"
)

// Registry holds all notification module repositories
type Registry struct {
	notificationChannel    *NotificationChannelRepository
	notificationPreference *NotificationPreferenceRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		notificationChannel:    NewNotificationChannelRepository(db),
		notificationPreference: NewNotificationPreferenceRepository(db),
	}
}

// NotificationChannel returns the notification channel repository
func (r *Registry) NotificationChannel() *NotificationChannelRepository { return r.notificationChannel }

// NotificationPreference returns the notification preference repository
func (r *Registry) NotificationPreference() *NotificationPreferenceRepository {
	return r.notificationPreference
}
