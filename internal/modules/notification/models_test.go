package notification

import (
	"encoding/json"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

func TestChannelData_Scan(t *testing.T) {
	tests := []struct {
		name        string
		value       interface{}
		expectErr   bool
		expectEmail string
	}{
		{
			name:        "scan json bytes",
			value:       []byte(`{"email":"test@example.com","appDeploy":true}`),
			expectErr:   false,
			expectEmail: "test@example.com",
		},
		{
			name:        "scan json string",
			value:       `{"email":"user@example.com"}`,
			expectErr:   false,
			expectEmail: "user@example.com",
		},
		{
			name:      "scan nil",
			value:     nil,
			expectErr: false,
		},
		{
			name:      "scan invalid type",
			value:     123,
			expectErr: true,
		},
		{
			name:      "scan invalid json",
			value:     []byte(`{invalid`),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cd models.ChannelData
			err := cd.Scan(tt.value)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.expectEmail != "" && cd.Email != tt.expectEmail {
				t.Errorf("ChannelData.Email = %v, want %v", cd.Email, tt.expectEmail)
			}
		})
	}
}

func TestChannelData_Value(t *testing.T) {
	cd := models.ChannelData{
		Email:      "test@example.com",
		WebhookURL: "https://example.com/webhook",
		AppDeploy:  true,
	}

	val, err := cd.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	bytes, ok := val.([]byte)
	if !ok {
		t.Error("expected []byte value")
		return
	}

	var decoded models.ChannelData
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Errorf("failed to unmarshal: %v", err)
		return
	}

	if decoded.Email != cd.Email {
		t.Errorf("decoded Email = %v, want %v", decoded.Email, cd.Email)
	}
	if decoded.WebhookURL != cd.WebhookURL {
		t.Errorf("decoded WebhookURL = %v, want %v", decoded.WebhookURL, cd.WebhookURL)
	}
}

func TestChannelData_ToChannelsData(t *testing.T) {
	cd := models.ChannelData{
		Email:          "test@example.com",
		WebhookURL:     "https://hooks.slack.com/xxx",
		BotToken:       "123456:ABC",
		ChatID:         "-123456789",
		AppDeploy:      true,
		DatabaseBackup: false,
	}

	result := cd.ToChannelsData()

	if result.Email != cd.Email {
		t.Errorf("Email = %v, want %v", result.Email, cd.Email)
	}
	if result.WebhookURL != cd.WebhookURL {
		t.Errorf("WebhookURL = %v, want %v", result.WebhookURL, cd.WebhookURL)
	}
	if result.BotToken != cd.BotToken {
		t.Errorf("BotToken = %v, want %v", result.BotToken, cd.BotToken)
	}
	if result.ChatID != cd.ChatID {
		t.Errorf("ChatID = %v, want %v", result.ChatID, cd.ChatID)
	}
	if result.AppDeploy != cd.AppDeploy {
		t.Errorf("AppDeploy = %v, want %v", result.AppDeploy, cd.AppDeploy)
	}
	if result.DatabaseBackup != cd.DatabaseBackup {
		t.Errorf("DatabaseBackup = %v, want %v", result.DatabaseBackup, cd.DatabaseBackup)
	}
}

func TestFromChannelsData(t *testing.T) {
	cd := channels.ChannelData{
		Email:          "test@example.com",
		WebhookURL:     "https://hooks.slack.com/xxx",
		BotToken:       "123456:ABC",
		ChatID:         "-123456789",
		AppDeploy:      true,
		DatabaseBackup: true,
	}

	result := models.FromChannelsData(cd)

	if result.Email != cd.Email {
		t.Errorf("Email = %v, want %v", result.Email, cd.Email)
	}
	if result.WebhookURL != cd.WebhookURL {
		t.Errorf("WebhookURL = %v, want %v", result.WebhookURL, cd.WebhookURL)
	}
}

func TestNotificationChannel_TableName(t *testing.T) {
	nc := models.NotificationChannel{}
	if got := nc.TableName(); got != "notification_channels" {
		t.Errorf("TableName() = %v, want notification_channels", got)
	}
}

func TestNotificationChannel_GetEmail(t *testing.T) {
	nc := models.NotificationChannel{
		Data: models.ChannelData{Email: "test@example.com"},
	}
	if got := nc.GetEmail(); got != "test@example.com" {
		t.Errorf("GetEmail() = %v, want test@example.com", got)
	}
}

func TestNotificationChannel_GetWebhookURL(t *testing.T) {
	nc := models.NotificationChannel{
		Data: models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
	}
	if got := nc.GetWebhookURL(); got != "https://hooks.slack.com/xxx" {
		t.Errorf("GetWebhookURL() = %v, want https://hooks.slack.com/xxx", got)
	}
}

func TestNotificationChannel_GetTelegramBotToken(t *testing.T) {
	nc := models.NotificationChannel{
		Data: models.ChannelData{BotToken: "123456:ABC"},
	}
	if got := nc.GetTelegramBotToken(); got != "123456:ABC" {
		t.Errorf("GetTelegramBotToken() = %v, want 123456:ABC", got)
	}
}

func TestNotificationChannel_GetTelegramChatID(t *testing.T) {
	nc := models.NotificationChannel{
		Data: models.ChannelData{ChatID: "-123456789"},
	}
	if got := nc.GetTelegramChatID(); got != "-123456789" {
		t.Errorf("GetTelegramChatID() = %v, want -123456789", got)
	}
}

func TestNotificationChannel_ToChannelsNotificationChannel(t *testing.T) {
	nc := models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeEmail,
		Label:     "Test Email",
		Data:      models.ChannelData{Email: "test@example.com"},
		Connected: true,
		IsDefault: false,
	}

	result := nc.ToChannelsNotificationChannel()

	if result.ID != nc.ID {
		t.Errorf("ID = %v, want %v", result.ID, nc.ID)
	}
	if result.Provider != channels.ChannelTypeEmail {
		t.Errorf("Provider = %v, want email", result.Provider)
	}
	if result.Data.Email != nc.Data.Email {
		t.Errorf("Data.Email = %v, want %v", result.Data.Email, nc.Data.Email)
	}
}

func TestBaseNotification(t *testing.T) {
	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Server has been provisioned")

	t.Run("Type", func(t *testing.T) {
		if got := notif.Type(); got != enums.NotificationTypeServerProvisioned {
			t.Errorf("Type() = %v, want %v", got, enums.NotificationTypeServerProvisioned)
		}
	})

	t.Run("RawText", func(t *testing.T) {
		if got := notif.RawText(); got != "Server has been provisioned" {
			t.Errorf("RawText() = %v, want 'Server has been provisioned'", got)
		}
	})

	t.Run("ToEmail", func(t *testing.T) {
		msg := notif.ToEmail()
		if msg.Subject != "Notification" {
			t.Errorf("ToEmail().Subject = %v, want 'Notification'", msg.Subject)
		}
		if msg.Body != "Server has been provisioned" {
			t.Errorf("ToEmail().Body = %v, want 'Server has been provisioned'", msg.Body)
		}
		if msg.IsHTML {
			t.Error("ToEmail().IsHTML should be false")
		}
	})

	t.Run("ToSlack", func(t *testing.T) {
		if got := notif.ToSlack(); got != "Server has been provisioned" {
			t.Errorf("ToSlack() = %v, want 'Server has been provisioned'", got)
		}
	})

	t.Run("ToDiscord", func(t *testing.T) {
		if got := notif.ToDiscord(); got != "Server has been provisioned" {
			t.Errorf("ToDiscord() = %v, want 'Server has been provisioned'", got)
		}
	})

	t.Run("ToTelegram", func(t *testing.T) {
		if got := notif.ToTelegram(); got != "Server has been provisioned" {
			t.Errorf("ToTelegram() = %v, want 'Server has been provisioned'", got)
		}
	})
}

func TestEmailMessage(t *testing.T) {
	msg := &channels.EmailMessage{
		Subject: "Test Subject",
		Body:    "<p>HTML Body</p>",
		IsHTML:  true,
	}

	if msg.Subject != "Test Subject" {
		t.Errorf("Subject = %v, want 'Test Subject'", msg.Subject)
	}
	if msg.Body != "<p>HTML Body</p>" {
		t.Errorf("Body = %v, want '<p>HTML Body</p>'", msg.Body)
	}
	if !msg.IsHTML {
		t.Error("IsHTML should be true")
	}
}
