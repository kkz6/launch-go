package channels

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestSlackChannel_Send(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		statusCode int
		httpError  error
		expectErr  bool
	}{
		{"successful send", "https://hooks.slack.com/xxx", 200, nil, false},
		{"empty webhook", "", 0, nil, true},
		{"http error", "https://hooks.slack.com/xxx", 0, errors.New("network error"), true},
		{"server error", "https://hooks.slack.com/xxx", 500, nil, true},
		{"redirect", "https://hooks.slack.com/xxx", 302, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, tt.statusCode, tt.httpError
				},
			}

			nc := &NotificationChannel{
				Provider: ChannelTypeSlack,
				Data:     ChannelData{WebhookURL: tt.webhookURL},
			}
			channel := NewSlackChannel(nc, mockHTTP)

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

func TestSlackChannel_Send_NilHTTPClient(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeSlack,
		Data:     ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
	}
	channel := NewSlackChannel(nc, nil)

	notif := NewMockNotification("Test notification")

	err := channel.Send(context.Background(), notif)
	if err == nil {
		t.Error("expected error for nil HTTP client")
	}
}

func TestSlackChannel_Connect(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		statusCode int
		httpError  error
		expectErr  bool
	}{
		{"successful connect", "https://hooks.slack.com/xxx", 200, nil, false},
		{"empty webhook", "", 0, nil, true},
		{"http error", "https://hooks.slack.com/xxx", 0, errors.New("network error"), true},
		{"server error", "https://hooks.slack.com/xxx", 500, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, tt.statusCode, tt.httpError
				},
			}

			nc := &NotificationChannel{
				Provider: ChannelTypeSlack,
				Data:     ChannelData{WebhookURL: tt.webhookURL},
			}
			channel := NewSlackChannel(nc, mockHTTP)

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

func TestSlackChannel_Connect_NilHTTPClient(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeSlack,
		Data:     ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
	}
	channel := NewSlackChannel(nc, nil)

	err := channel.Connect(context.Background())
	if err == nil {
		t.Error("expected error for nil HTTP client")
	}
}

func TestSlackChannel_GetCreateRules(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeSlack,
	}
	channel := NewSlackChannel(nc, nil)

	rules := channel.GetCreateRules()

	if rules["webhook_url"] != "required,url" {
		t.Errorf("webhook_url rule = %s, want 'required,url'", rules["webhook_url"])
	}
}

func TestSlackChannel_GetData(t *testing.T) {
	nc := &NotificationChannel{
		Provider: ChannelTypeSlack,
		Data: ChannelData{
			WebhookURL: "https://hooks.slack.com/xxx",
			AppDeploy:  true,
		},
	}
	channel := NewSlackChannel(nc, nil)

	data := channel.GetData()

	if data.WebhookURL != "https://hooks.slack.com/xxx" {
		t.Errorf("WebhookURL = %s, want 'https://hooks.slack.com/xxx'", data.WebhookURL)
	}
	if !data.AppDeploy {
		t.Error("AppDeploy should be true")
	}
}

func TestSlackChannel_Send_VerifiesPayload(t *testing.T) {
	var receivedBody interface{}

	mockHTTP := &MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			receivedBody = body
			return nil, http.StatusOK, nil
		},
	}

	nc := &NotificationChannel{
		Provider: ChannelTypeSlack,
		Data:     ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
	}
	channel := NewSlackChannel(nc, mockHTTP)

	notif := NewMockNotification("Test message content")

	_ = channel.Send(context.Background(), notif)

	msg, ok := receivedBody.(slackMessage)
	if !ok {
		t.Error("expected slackMessage type")
		return
	}

	if msg.Text != "Test message content" {
		t.Errorf("Text = %s, want 'Test message content'", msg.Text)
	}
}

func TestNewSlackChannel(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	nc := &NotificationChannel{
		Provider: ChannelTypeSlack,
	}

	channel := NewSlackChannel(nc, mockHTTP)

	if channel == nil {
		t.Fatal("NewSlackChannel() returned nil")
	}
	if channel.channel != nc {
		t.Error("channel not set correctly")
	}
	if channel.httpClient != mockHTTP {
		t.Error("httpClient not set correctly")
	}
}
