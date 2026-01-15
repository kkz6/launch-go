package dto

import (
	"strconv"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

// ChannelResponse represents the response for a notification channel
type ChannelResponse struct {
	ID        string              `json:"id"`
	UserID    string              `json:"user_id"`
	TeamID    string              `json:"team_id"`
	Provider  string              `json:"provider"`
	Label     string              `json:"label"`
	Data      ChannelDataResponse `json:"data"`
	Connected bool                `json:"connected"`
	IsDefault bool                `json:"is_default"`
	CreatedAt string              `json:"created_at"`
	UpdatedAt string              `json:"updated_at"`
}

// ChannelDataResponse represents the data response for a notification channel
type ChannelDataResponse struct {
	Email          string `json:"email,omitempty"`
	WebhookURL     string `json:"webhook_url,omitempty"`
	BotToken       string `json:"bot_token,omitempty"`
	ChatID         string `json:"chat_id,omitempty"`
	AppDeploy      bool   `json:"appDeploy"`
	DatabaseBackup bool   `json:"databaseBackup"`
}

// ListChannelsResponse represents the response for listing channels
type ListChannelsResponse struct {
	Channels []ChannelResponse `json:"channels"`
}

// ChannelTypeResponse represents an available notification channel type
type ChannelTypeResponse struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

// ToChannelResponse converts a NotificationChannel to a ChannelResponse
func ToChannelResponse(channel *models.NotificationChannel) ChannelResponse {
	createdAt := ""
	if channel.CreatedAt != nil {
		createdAt = channel.CreatedAt.Format(time.RFC3339)
	}

	updatedAt := ""
	if channel.UpdatedAt != nil {
		updatedAt = channel.UpdatedAt.Format(time.RFC3339)
	}

	return ChannelResponse{
		ID:       strconv.FormatUint(channel.ID, 10),
		UserID:   channel.UserID,
		TeamID:   channel.TeamID,
		Provider: channel.Provider.String(),
		Label:    channel.Label,
		Data: ChannelDataResponse{
			Email:          channel.Data.Email,
			WebhookURL:     channel.Data.WebhookURL,
			BotToken:       maskToken(channel.Data.BotToken),
			ChatID:         channel.Data.ChatID,
			AppDeploy:      channel.Data.AppDeploy,
			DatabaseBackup: channel.Data.DatabaseBackup,
		},
		Connected: channel.Connected,
		IsDefault: channel.IsDefault,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

// ToChannelResponses converts a slice of NotificationChannels to ChannelResponses
func ToChannelResponses(channels []models.NotificationChannel) []ChannelResponse {
	responses := make([]ChannelResponse, len(channels))
	for i, channel := range channels {
		responses[i] = ToChannelResponse(&channel)
	}

	return responses
}

// maskToken masks sensitive tokens for display
func maskToken(token string) string {
	if token == "" {
		return ""
	}

	if len(token) <= 8 {
		return "****"
	}

	return token[:4] + "****" + token[len(token)-4:]
}
