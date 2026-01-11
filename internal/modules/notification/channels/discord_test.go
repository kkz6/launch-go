package channels

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestDiscordChannel_Send(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		statusCode int
		httpError  error
		expectErr  bool
	}{
		{"successful send", "https://discord.com/api/webhooks/xxx", 200, nil, false},
		{"empty webhook", "", 0, nil, true},
		{"http error", "https://discord.com/api/webhooks/xxx", 0, errors.New("network error"), true},
		{"server error", "https://discord.com/api/webhooks/xxx", 500, nil, true},
		{"redirect", "https://discord.com/api/webhooks/xxx", 302, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, tt.statusCode, tt.httpError
				},
			}

			nc := &NotificationChannel{
				Provider: ChannelTypeDiscord,
				Data:     ChannelData{WebhookURL: tt.webhookURL},
			}
			channel := NewDiscordChannel(nc, mockHTTP)

			notif := NewMockNotification("Test notification")

			err := channel.Send(context.Background(), notif)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDiscordChannel_Send_NilHTTPClient(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeDiscord,
		Data:     ChannelData{WebhookURL: "https://discord.com/api/webhooks/xxx"},
	}
	channel := NewDiscordChannel(nc, nil)

	notif := NewMockNotification("Test notification")

	err := channel.Send(context.Background(), notif)
	if err == nil {
		t.Error("expected error for nil HTTP client")
	}
}

func TestDiscordChannel_Connect(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		statusCode int
		httpError  error
		expectErr  bool
	}{
		{"successful connect", "https://discord.com/api/webhooks/xxx", 200, nil, false},
		{"empty webhook", "", 0, nil, true},
		{"http error", "https://discord.com/api/webhooks/xxx", 0, errors.New("network error"), true},
		{"server error", "https://discord.com/api/webhooks/xxx", 500, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, tt.statusCode, tt.httpError
				},
			}

			nc := &NotificationChannel{
				Provider: ChannelTypeDiscord,
				Data:     ChannelData{WebhookURL: tt.webhookURL},
			}
			channel := NewDiscordChannel(nc, mockHTTP)

			err := channel.Connect(context.Background())

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDiscordChannel_Connect_NilHTTPClient(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeDiscord,
		Data:     ChannelData{WebhookURL: "https://discord.com/api/webhooks/xxx"},
	}
	channel := NewDiscordChannel(nc, nil)

	err := channel.Connect(context.Background())
	if err == nil {
		t.Error("expected error for nil HTTP client")
	}
}

func TestDiscordChannel_GetCreateRules(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeDiscord,
	}
	channel := NewDiscordChannel(nc, nil)

	rules := channel.GetCreateRules()

	if rules["webhook_url"] != "required,url" {
		t.Errorf("webhook_url rule = %s, want 'required,url'", rules["webhook_url"])
	}
}

func TestDiscordChannel_GetData(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeDiscord,
		Data: ChannelData{
			WebhookURL:     "https://discord.com/api/webhooks/xxx",
			DatabaseBackup: true,
		},
	}
	channel := NewDiscordChannel(nc, nil)

	data := channel.GetData()

	if data.WebhookURL != "https://discord.com/api/webhooks/xxx" {
		t.Errorf("WebhookURL = %s, want 'https://discord.com/api/webhooks/xxx'", data.WebhookURL)
	}
	if !data.DatabaseBackup {
		t.Error("DatabaseBackup should be true")
	}
}

func TestDiscordChannel_Send_VerifiesPayload(t *testing.T) {
	var receivedBody interface{}

	mockHTTP := &MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			receivedBody = body
			return nil, http.StatusOK, nil
		},
	}

	nc := &NotificationChannel{
		Provider: ChannelTypeDiscord,
		Data:     ChannelData{WebhookURL: "https://discord.com/api/webhooks/xxx"},
	}
	channel := NewDiscordChannel(nc, mockHTTP)

	notif := NewMockNotification("Discord test message")

	_ = channel.Send(context.Background(), notif)

	msg, ok := receivedBody.(discordMessage)
	if !ok {
		t.Error("expected discordMessage type")
		return
	}

	if msg.Content != "Discord test message" {
		t.Errorf("Content = %s, want 'Discord test message'", msg.Content)
	}
}

func TestNewDiscordChannel(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	nc := &NotificationChannel{
		Provider: ChannelTypeDiscord,
	}

	channel := NewDiscordChannel(nc, mockHTTP)

	if channel == nil {
		t.Error("NewDiscordChannel() returned nil")
	}
	if channel.channel != nc {
		t.Error("channel not set correctly")
	}
	if channel.httpClient != mockHTTP {
		t.Error("httpClient not set correctly")
	}
}
