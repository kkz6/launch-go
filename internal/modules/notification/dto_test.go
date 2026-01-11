package notification

import (
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

func TestCreateChannelRequest_ToChannelData(t *testing.T) {
	req := &dto.CreateChannelRequest{
		Provider:       "email",
		Label:          "Test Email",
		Email:          "test@example.com",
		WebhookURL:     "https://example.com/webhook",
		BotToken:       "123456:ABC",
		ChatID:         "-123456789",
		AppDeploy:      true,
		DatabaseBackup: false,
	}

	data := req.ToChannelData()

	if data.Email != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", data.Email)
	}
	if data.WebhookURL != "https://example.com/webhook" {
		t.Errorf("WebhookURL = %v, want https://example.com/webhook", data.WebhookURL)
	}
	if data.BotToken != "123456:ABC" {
		t.Errorf("BotToken = %v, want 123456:ABC", data.BotToken)
	}
	if data.ChatID != "-123456789" {
		t.Errorf("ChatID = %v, want -123456789", data.ChatID)
	}
	if !data.AppDeploy {
		t.Error("AppDeploy should be true")
	}
	if data.DatabaseBackup {
		t.Error("DatabaseBackup should be false")
	}
}

func TestUpdateChannelRequest_ToChannelData(t *testing.T) {
	req := &dto.UpdateChannelRequest{
		Label:          "Updated Email",
		Email:          "updated@example.com",
		AppDeploy:      true,
		DatabaseBackup: true,
	}

	data := req.ToChannelData()

	if data.Email != "updated@example.com" {
		t.Errorf("Email = %v, want updated@example.com", data.Email)
	}
	if !data.AppDeploy {
		t.Error("AppDeploy should be true")
	}
	if !data.DatabaseBackup {
		t.Error("DatabaseBackup should be true")
	}
}

func TestToChannelResponse(t *testing.T) {
	now := time.Now()
	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "01HXYZ987654321ZYXWVUTSRQ",
		TeamID:    "01HXYZTEAM123456789ABCDEF",
		Provider:  enums.ChannelTypeEmail,
		Label:     "My Email",
		Data: models.ChannelData{
			Email:          "test@example.com",
			BotToken:       "1234567890:ABCDEFGHIJKLMNOP",
			AppDeploy:      true,
			DatabaseBackup: false,
		},
		Connected: true,
		IsDefault: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	resp := dto.ToChannelResponse(channel)

	if resp.ID != channel.ID {
		t.Errorf("ID = %v, want %v", resp.ID, channel.ID)
	}
	if resp.UserID != channel.UserID {
		t.Errorf("UserID = %v, want %v", resp.UserID, channel.UserID)
	}
	if resp.TeamID != channel.TeamID {
		t.Errorf("TeamID = %v, want %v", resp.TeamID, channel.TeamID)
	}
	if resp.Provider != "email" {
		t.Errorf("Provider = %v, want email", resp.Provider)
	}
	if resp.Label != "My Email" {
		t.Errorf("Label = %v, want 'My Email'", resp.Label)
	}
	if !resp.Connected {
		t.Error("Connected should be true")
	}
	if resp.IsDefault {
		t.Error("IsDefault should be false")
	}
	if resp.Data.Email != "test@example.com" {
		t.Errorf("Data.Email = %v, want test@example.com", resp.Data.Email)
	}
	// Bot token should be masked
	if resp.Data.BotToken != "1234****MNOP" {
		t.Errorf("Data.BotToken = %v, want 1234****MNOP", resp.Data.BotToken)
	}
}

func TestToChannelResponses(t *testing.T) {
	now := time.Now()
	channels := []models.NotificationChannel{
		{
			ID:        "01HXYZ123456789ABCDEFGHIJ",
			Provider:  enums.ChannelTypeEmail,
			Label:     "Email 1",
			Connected: true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "01HXYZ123456789ABCDEFGHIK",
			Provider:  enums.ChannelTypeSlack,
			Label:     "Slack 1",
			Connected: true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	responses := dto.ToChannelResponses(channels)

	if len(responses) != 2 {
		t.Errorf("len(responses) = %d, want 2", len(responses))
	}
	if responses[0].Label != "Email 1" {
		t.Errorf("responses[0].Label = %v, want 'Email 1'", responses[0].Label)
	}
	if responses[1].Label != "Slack 1" {
		t.Errorf("responses[1].Label = %v, want 'Slack 1'", responses[1].Label)
	}
}

func TestChannelDataResponse(t *testing.T) {
	resp := dto.ChannelDataResponse{
		Email:          "test@example.com",
		WebhookURL:     "https://hooks.slack.com/xxx",
		BotToken:       "masked",
		ChatID:         "-123456",
		AppDeploy:      true,
		DatabaseBackup: true,
	}

	if resp.Email != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", resp.Email)
	}
	if resp.WebhookURL != "https://hooks.slack.com/xxx" {
		t.Errorf("WebhookURL = %v, want https://hooks.slack.com/xxx", resp.WebhookURL)
	}
}

func TestListChannelsResponse(t *testing.T) {
	resp := dto.ListChannelsResponse{
		Channels: []dto.ChannelResponse{
			{ID: "1", Label: "Channel 1"},
			{ID: "2", Label: "Channel 2"},
		},
	}

	if len(resp.Channels) != 2 {
		t.Errorf("len(Channels) = %d, want 2", len(resp.Channels))
	}
}

func TestSendNotificationRequest(t *testing.T) {
	req := dto.SendNotificationRequest{
		TeamID:           "team123",
		NotificationType: "deployment_failed",
		Title:            "Deployment Failed",
		Message:          "Deployment to production failed",
	}

	if req.TeamID != "team123" {
		t.Errorf("TeamID = %v, want team123", req.TeamID)
	}
	if req.NotificationType != "deployment_failed" {
		t.Errorf("NotificationType = %v, want deployment_failed", req.NotificationType)
	}
}

func TestTestChannelRequest(t *testing.T) {
	req := dto.TestChannelRequest{
		Message: "Test message",
	}

	if req.Message != "Test message" {
		t.Errorf("Message = %v, want 'Test message'", req.Message)
	}
}
