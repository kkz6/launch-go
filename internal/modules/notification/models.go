package notification

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// ChannelData represents the JSON data stored in the notification channel
type ChannelData struct {
	// Email channel
	Email string `json:"email,omitempty"`

	// Slack channel
	WebhookURL string `json:"webhook_url,omitempty"`

	// Telegram channel
	BotToken string `json:"bot_token,omitempty"`
	ChatID   string `json:"chat_id,omitempty"`

	// Common preferences
	AppDeploy      bool `json:"appDeploy"`
	DatabaseBackup bool `json:"databaseBackup"`
}

// Scan implements the sql.Scanner interface for ChannelData
func (cd *ChannelData) Scan(value interface{}) error {
	if value == nil {
		*cd = ChannelData{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	return json.Unmarshal(bytes, cd)
}

// Value implements the driver.Valuer interface for ChannelData
func (cd ChannelData) Value() (driver.Value, error) {
	return json.Marshal(cd)
}

// ToChannelsData converts to the channels package ChannelData
func (cd ChannelData) ToChannelsData() channels.ChannelData {
	return channels.ChannelData{
		Email:          cd.Email,
		WebhookURL:     cd.WebhookURL,
		BotToken:       cd.BotToken,
		ChatID:         cd.ChatID,
		AppDeploy:      cd.AppDeploy,
		DatabaseBackup: cd.DatabaseBackup,
	}
}

// FromChannelsData converts from the channels package ChannelData
func FromChannelsData(cd channels.ChannelData) ChannelData {
	return ChannelData{
		Email:          cd.Email,
		WebhookURL:     cd.WebhookURL,
		BotToken:       cd.BotToken,
		ChatID:         cd.ChatID,
		AppDeploy:      cd.AppDeploy,
		DatabaseBackup: cd.DatabaseBackup,
	}
}

// NotificationChannel represents a notification channel configuration
type NotificationChannel struct {
	ID        string         `gorm:"primaryKey;size:26" json:"id"`
	UserID    string         `gorm:"size:26;not null;index" json:"user_id"`
	TeamID    string         `gorm:"size:26;not null;index" json:"team_id"`
	Provider  ChannelType    `gorm:"size:50;not null" json:"provider"`
	Label     string         `gorm:"size:255;not null" json:"label"`
	Data      ChannelData    `gorm:"type:json" json:"data"`
	Connected bool           `gorm:"default:false" json:"connected"`
	IsDefault bool           `gorm:"default:false" json:"is_default"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for the NotificationChannel model
func (NotificationChannel) TableName() string {
	return "notification_channels"
}

// BeforeCreate is a GORM hook to generate ULID before creating
func (nc *NotificationChannel) BeforeCreate(tx *gorm.DB) error {
	if nc.ID == "" {
		nc.ID = utils.NewULID()
	}
	return nil
}

// GetEmail returns the email from channel data
func (nc *NotificationChannel) GetEmail() string {
	return nc.Data.Email
}

// GetWebhookURL returns the webhook URL from channel data
func (nc *NotificationChannel) GetWebhookURL() string {
	return nc.Data.WebhookURL
}

// GetTelegramBotToken returns the Telegram bot token from channel data
func (nc *NotificationChannel) GetTelegramBotToken() string {
	return nc.Data.BotToken
}

// GetTelegramChatID returns the Telegram chat ID from channel data
func (nc *NotificationChannel) GetTelegramChatID() string {
	return nc.Data.ChatID
}

// ToChannelsNotificationChannel converts to the channels package NotificationChannel
func (nc *NotificationChannel) ToChannelsNotificationChannel() *channels.NotificationChannel {
	return &channels.NotificationChannel{
		ID:        nc.ID,
		UserID:    nc.UserID,
		TeamID:    nc.TeamID,
		Provider:  channels.ChannelType(nc.Provider),
		Label:     nc.Label,
		Data:      nc.Data.ToChannelsData(),
		Connected: nc.Connected,
		IsDefault: nc.IsDefault,
	}
}

// Notification represents an abstract notification that can be sent through channels
type Notification interface {
	// Type returns the notification type
	Type() NotificationType

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
	notificationType NotificationType
	rawText          string
}

// NewBaseNotification creates a new base notification
func NewBaseNotification(notificationType NotificationType, rawText string) *BaseNotification {
	return &BaseNotification{
		notificationType: notificationType,
		rawText:          rawText,
	}
}

// Type returns the notification type
func (n *BaseNotification) Type() NotificationType {
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
