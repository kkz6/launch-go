package models

import (
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
)

// Notification represents an abstract notification that can be sent through channels
type Notification interface {
	// Type returns the notification type
	Type() enums.NotificationType

	// RawText returns the plain text representation
	RawText() string

	// ToEmail returns the email message content
	ToEmail() *channels.EmailMessage

	// ToSlack returns the Slack message content
	ToSlack() string

	// ToDiscord returns the Discord message content
	ToDiscord() string

	// ToTelegram returns the Telegram message content
	ToTelegram() string
}

// BaseNotification provides a default implementation for notifications
type BaseNotification struct {
	notificationType enums.NotificationType
	rawText          string
}

// NewBaseNotification creates a new base notification
func NewBaseNotification(notificationType enums.NotificationType, rawText string) *BaseNotification {
	return &BaseNotification{
		notificationType: notificationType,
		rawText:          rawText,
	}
}

// Type returns the notification type
func (n *BaseNotification) Type() enums.NotificationType {
	return n.notificationType
}

// RawText returns the plain text representation
func (n *BaseNotification) RawText() string {
	return n.rawText
}

// ToEmail returns the email message content
func (n *BaseNotification) ToEmail() *channels.EmailMessage {
	return &channels.EmailMessage{
		Subject: "Notification",
		Body:    n.rawText,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *BaseNotification) ToSlack() string {
	return n.rawText
}

// ToDiscord returns the Discord message content
func (n *BaseNotification) ToDiscord() string {
	return n.rawText
}

// ToTelegram returns the Telegram message content
func (n *BaseNotification) ToTelegram() string {
	return n.rawText
}

// Notifiable represents an entity that can receive notifications
type Notifiable interface {
	// GetNotificationChannels returns all notification channels for the notifiable
	GetNotificationChannels() []NotificationChannel
}
