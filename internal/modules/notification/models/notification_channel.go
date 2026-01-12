package models

import (
	"strconv"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
)

// NotificationChannel represents a notification channel configuration
// Note: Uses auto-increment ID
type NotificationChannel struct {
	ID        uint64            `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    string            `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID    string            `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	Provider  enums.ChannelType `gorm:"type:varchar(255);not null" json:"provider"`
	Label     string            `gorm:"type:varchar(255);not null" json:"label"`
	Data      ChannelData       `gorm:"type:json" json:"data"`
	Connected bool              `gorm:"default:false" json:"connected"`
	IsDefault bool              `gorm:"column:is_default;default:false" json:"is_default"`
	CreatedAt *time.Time        `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt *time.Time        `gorm:"type:timestamp null" json:"updated_at,omitempty"`
}

// TableName returns the table name for the NotificationChannel model
func (NotificationChannel) TableName() string {
	return "notification_channels"
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
		ID:        strconv.FormatUint(nc.ID, 10),
		UserID:    nc.UserID,
		TeamID:    nc.TeamID,
		Provider:  channels.ChannelType(nc.Provider),
		Label:     nc.Label,
		Data:      nc.Data.ToChannelsData(),
		Connected: nc.Connected,
		IsDefault: nc.IsDefault,
	}
}
