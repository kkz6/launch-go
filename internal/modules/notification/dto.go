package notification

import "time"

// CreateChannelRequest represents the request to create a notification channel
type CreateChannelRequest struct {
	Provider       string `json:"provider" validate:"required,oneof=email slack discord telegram"`
	Label          string `json:"label" validate:"required,min=1,max=255"`
	Email          string `json:"email" validate:"required_if=Provider email,omitempty,email"`
	WebhookURL     string `json:"webhook_url" validate:"required_if=Provider slack,required_if=Provider discord,omitempty,url"`
	BotToken       string `json:"bot_token" validate:"required_if=Provider telegram"`
	ChatID         string `json:"chat_id" validate:"required_if=Provider telegram"`
	AppDeploy      bool   `json:"appDeploy"`
	DatabaseBackup bool   `json:"databaseBackup"`
}

// UpdateChannelRequest represents the request to update a notification channel
type UpdateChannelRequest struct {
	Label          string `json:"label" validate:"required,min=1,max=255"`
	Email          string `json:"email" validate:"omitempty,email"`
	WebhookURL     string `json:"webhook_url" validate:"omitempty,url"`
	BotToken       string `json:"bot_token"`
	ChatID         string `json:"chat_id"`
	AppDeploy      bool   `json:"appDeploy"`
	DatabaseBackup bool   `json:"databaseBackup"`
}

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

// ToChannelResponse converts a NotificationChannel to a ChannelResponse
func ToChannelResponse(channel *NotificationChannel) ChannelResponse {
	return ChannelResponse{
		ID:        channel.ID,
		UserID:    channel.UserID,
		TeamID:    channel.TeamID,
		Provider:  channel.Provider.String(),
		Label:     channel.Label,
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
		CreatedAt: channel.CreatedAt.Format(time.RFC3339),
		UpdatedAt: channel.UpdatedAt.Format(time.RFC3339),
	}
}

// ToChannelResponses converts a slice of NotificationChannels to ChannelResponses
func ToChannelResponses(channels []NotificationChannel) []ChannelResponse {
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

// ToChannelData converts a CreateChannelRequest to ChannelData
func (r *CreateChannelRequest) ToChannelData() ChannelData {
	return ChannelData{
		Email:          r.Email,
		WebhookURL:     r.WebhookURL,
		BotToken:       r.BotToken,
		ChatID:         r.ChatID,
		AppDeploy:      r.AppDeploy,
		DatabaseBackup: r.DatabaseBackup,
	}
}

// ToChannelData converts an UpdateChannelRequest to ChannelData
func (r *UpdateChannelRequest) ToChannelData() ChannelData {
	return ChannelData{
		Email:          r.Email,
		WebhookURL:     r.WebhookURL,
		BotToken:       r.BotToken,
		ChatID:         r.ChatID,
		AppDeploy:      r.AppDeploy,
		DatabaseBackup: r.DatabaseBackup,
	}
}

// SendNotificationRequest represents a request to send a notification
type SendNotificationRequest struct {
	TeamID           string `json:"team_id" validate:"required"`
	NotificationType string `json:"notification_type" validate:"required"`
	Title            string `json:"title" validate:"required"`
	Message          string `json:"message" validate:"required"`
}

// TestChannelRequest represents a request to test a notification channel
type TestChannelRequest struct {
	Message string `json:"message" validate:"omitempty,max=500"`
}
