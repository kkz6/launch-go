package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
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
