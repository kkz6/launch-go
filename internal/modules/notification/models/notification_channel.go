package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// NotificationChannel represents a notification channel configuration
type NotificationChannel struct {
	ID        string           `gorm:"primaryKey;size:26" json:"id"`
	UserID    string           `gorm:"size:26;not null;index" json:"user_id"`
	TeamID    string           `gorm:"size:26;not null;index" json:"team_id"`
	Provider  enums.ChannelType `gorm:"size:50;not null" json:"provider"`
	Label     string           `gorm:"size:255;not null" json:"label"`
	Data      ChannelData      `gorm:"type:json" json:"data"`
	Connected bool             `gorm:"default:false" json:"connected"`
	IsDefault bool             `gorm:"default:false" json:"is_default"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	DeletedAt gorm.DeletedAt   `gorm:"index" json:"-"`
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
