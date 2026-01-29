package channels

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestTelegramChannel_Send(t *testing.T) {
	tests := []struct {
		name       string
		botToken   string
		chatID     string
		statusCode int
		httpError  error
		expectErr  bool
	}{
		{"successful send", "123456:ABC", "-123456789", 200, nil, false},
		{"empty bot token", "", "-123456789", 0, nil, true},
		{"empty chat id", "123456:ABC", "", 0, nil, true},
		{"both empty", "", "", 0, nil, true},
		{"http error", "123456:ABC", "-123456789", 0, errors.New("network error"), true},
		{"server error", "123456:ABC", "-123456789", 500, nil, true},
		{"redirect", "123456:ABC", "-123456789", 302, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, tt.statusCode, tt.httpError
				},
			}

			nc := &NotificationChannel{
				Provider: ChannelTypeTelegram,
				Data: ChannelData{
					BotToken: tt.botToken,
					ChatID:   tt.chatID,
				},
			}
			channel := NewTelegramChannel(nc, mockHTTP)

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

func TestTelegramChannel_Send_NilHTTPClient(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeTelegram,
		Data: ChannelData{
			BotToken: "123456:ABC",
			ChatID:   "-123456789",
		},
	}
	channel := NewTelegramChannel(nc, nil)

	notif := NewMockNotification("Test notification")

	err := channel.Send(context.Background(), notif)
	if err == nil {
		t.Error("expected error for nil HTTP client")
	}
}

func TestTelegramChannel_Connect(t *testing.T) {
	tests := []struct {
		name       string
		botToken   string
		chatID     string
		statusCode int
		httpError  error
		expectErr  bool
	}{
		{"successful connect", "123456:ABC", "-123456789", 200, nil, false},
		{"empty bot token", "", "-123456789", 0, nil, true},
		{"empty chat id", "123456:ABC", "", 0, nil, true},
		{"http error", "123456:ABC", "-123456789", 0, errors.New("network error"), true},
		{"server error", "123456:ABC", "-123456789", 500, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, tt.statusCode, tt.httpError
				},
			}

			nc := &NotificationChannel{
				Provider: ChannelTypeTelegram,
				Data: ChannelData{
					BotToken: tt.botToken,
					ChatID:   tt.chatID,
				},
			}
			channel := NewTelegramChannel(nc, mockHTTP)

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

func TestTelegramChannel_Connect_NilHTTPClient(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeTelegram,
		Data: ChannelData{
			BotToken: "123456:ABC",
			ChatID:   "-123456789",
		},
	}
	channel := NewTelegramChannel(nc, nil)

	err := channel.Connect(context.Background())
	if err == nil {
		t.Error("expected error for nil HTTP client")
	}
}

func TestTelegramChannel_GetCreateRules(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeTelegram,
	}
	channel := NewTelegramChannel(nc, nil)

	rules := channel.GetCreateRules()

	if rules["bot_token"] != "required" {
		t.Errorf("bot_token rule = %s, want 'required'", rules["bot_token"])
	}
	if rules["chat_id"] != "required" {
		t.Errorf("chat_id rule = %s, want 'required'", rules["chat_id"])
	}
}

func TestTelegramChannel_GetData(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeTelegram,
		Data: ChannelData{
			BotToken:  "123456:ABC",
			ChatID:    "-123456789",
			AppDeploy: true,
		},
	}
	channel := NewTelegramChannel(nc, nil)

	data := channel.GetData()

	if data.BotToken != "123456:ABC" {
		t.Errorf("BotToken = %s, want '123456:ABC'", data.BotToken)
	}
	if data.ChatID != "-123456789" {
		t.Errorf("ChatID = %s, want '-123456789'", data.ChatID)
	}
	if !data.AppDeploy {
		t.Error("AppDeploy should be true")
	}
}

func TestTelegramChannel_Send_VerifiesPayload(t *testing.T) {
	var receivedURL string
	var receivedBody interface{}

	mockHTTP := &MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			receivedURL = url
			receivedBody = body
			return nil, http.StatusOK, nil
		},
	}

	nc := &NotificationChannel{
		Provider: ChannelTypeTelegram,
		Data: ChannelData{
			BotToken: "123456:ABC",
			ChatID:   "-123456789",
		},
	}
	channel := NewTelegramChannel(nc, mockHTTP)

	notif := NewMockNotification("Telegram test message")

	_ = channel.Send(context.Background(), notif)

	// Verify URL contains bot token
	if !strings.Contains(receivedURL, "123456:ABC") {
		t.Errorf("URL should contain bot token, got: %s", receivedURL)
	}
	if !strings.Contains(receivedURL, "/sendMessage") {
		t.Errorf("URL should contain /sendMessage, got: %s", receivedURL)
	}

	msg, ok := receivedBody.(telegramMessage)
	if !ok {
		t.Error("expected telegramMessage type")
		return
	}

	if msg.ChatID != "-123456789" {
		t.Errorf("ChatID = %s, want '-123456789'", msg.ChatID)
	}
	if msg.Text != "Telegram test message" {
		t.Errorf("Text = %s, want 'Telegram test message'", msg.Text)
	}
	if msg.ParseMode != "HTML" {
		t.Errorf("ParseMode = %s, want 'HTML'", msg.ParseMode)
	}
}

func TestNewTelegramChannel(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	nc := &NotificationChannel{
		Provider: ChannelTypeTelegram,
	}

	channel := NewTelegramChannel(nc, mockHTTP)

	if channel == nil {
		t.Fatal("NewTelegramChannel() returned nil")
	}
	if channel.channel != nc {
		t.Error("channel not set correctly")
	}
	if channel.httpClient != mockHTTP {
		t.Error("httpClient not set correctly")
	}
	if channel.apiURL != telegramAPIURL {
		t.Errorf("apiURL = %s, want %s", channel.apiURL, telegramAPIURL)
	}
}

func TestNewTelegramChannelWithURL(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	nc := &NotificationChannel{
		Provider: ChannelTypeTelegram,
	}
	customURL := "https://custom-api.example.com/bot"

	channel := NewTelegramChannelWithURL(nc, mockHTTP, customURL)

	if channel == nil {
		t.Fatal("NewTelegramChannelWithURL() returned nil")
	}
	if channel.apiURL != customURL {
		t.Errorf("apiURL = %s, want %s", channel.apiURL, customURL)
	}
}

func TestTelegramChannel_Send_CustomAPIURL(t *testing.T) {
	var receivedURL string

	mockHTTP := &MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			receivedURL = url
			return nil, http.StatusOK, nil
		},
	}

	nc := &NotificationChannel{
		Provider: ChannelTypeTelegram,
		Data: ChannelData{
			BotToken: "test-token",
			ChatID:   "123",
		},
	}
	customURL := "https://custom-api.example.com/bot"
	channel := NewTelegramChannelWithURL(nc, mockHTTP, customURL)

	notif := NewMockNotification("Test")

	_ = channel.Send(context.Background(), notif)

	if !strings.HasPrefix(receivedURL, customURL) {
		t.Errorf("URL should start with custom URL, got: %s", receivedURL)
	}
}
