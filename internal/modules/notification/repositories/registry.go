package repositories

import (
	"gorm.io/gorm"
)

// Registry holds all notification module repositories
type Registry struct {
	notificationChannel *NotificationChannelRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		notificationChannel: NewNotificationChannelRepository(db),
	}
}

// NotificationChannel returns the notification channel repository
func (r *Registry) NotificationChannel() *NotificationChannelRepository { return r.notificationChannel }
