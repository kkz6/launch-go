package dto

import (
	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

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

// ToChannelData converts a CreateChannelRequest to ChannelData
func (r *CreateChannelRequest) ToChannelData() models.ChannelData {
	return models.ChannelData{
		Email:          r.Email,
		WebhookURL:     r.WebhookURL,
		BotToken:       r.BotToken,
		ChatID:         r.ChatID,
		AppDeploy:      r.AppDeploy,
		DatabaseBackup: r.DatabaseBackup,
	}
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

// ToChannelData converts an UpdateChannelRequest to ChannelData
func (r *UpdateChannelRequest) ToChannelData() models.ChannelData {
	return models.ChannelData{
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

// UpdateNotificationPreferencesRequest represents the request to update notification preferences
type UpdateNotificationPreferencesRequest struct {
	EmailServerCreated     bool `json:"email_server_created"`
	EmailServerDeleted     bool `json:"email_server_deleted"`
	EmailDeploymentSuccess bool `json:"email_deployment_success"`
	EmailDeploymentFailed  bool `json:"email_deployment_failed"`
	EmailBackupSuccess     bool `json:"email_backup_success"`
	EmailBackupFailed      bool `json:"email_backup_failed"`
}
