package channels

import (
	"testing"
)

func TestFactory_CreateChannel(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	factory := NewFactory(mockHTTP)

	tests := []struct {
		name       string
		provider   ChannelType
		expectErr  bool
		expectType string
	}{
		{"email channel", ChannelTypeEmail, false, "*channels.EmailChannel"},
		{"slack channel", ChannelTypeSlack, false, "*channels.SlackChannel"},
		{"discord channel", ChannelTypeDiscord, false, "*channels.DiscordChannel"},
		{"telegram channel", ChannelTypeTelegram, false, "*channels.TelegramChannel"},
		{"invalid channel", ChannelType("invalid"), true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nc := &NotificationChannel{
				Provider: tt.provider,
			}

			channel, err := factory.CreateChannel(nc)

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

			if channel == nil {
				t.Error("expected channel, got nil")
			}
		})
	}
}

func TestNewFactory(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	factory := NewFactory(mockHTTP)

	if factory == nil {
		t.Error("NewFactory() returned nil")
	}
	if factory.httpClient != mockHTTP {
		t.Error("httpClient not set correctly")
	}
}

func TestNewFactoryWithEmail(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	mockEmail := &MockEmailSender{}
	factory := NewFactoryWithEmail(mockHTTP, mockEmail)

	if factory == nil {
		t.Error("NewFactoryWithEmail() returned nil")
	}
	if factory.httpClient != mockHTTP {
		t.Error("httpClient not set correctly")
	}
	if factory.emailSender != mockEmail {
		t.Error("emailSender not set correctly")
	}
}

func TestFactory_CreateChannel_EmailWithSender(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	mockEmail := &MockEmailSender{}
	factory := NewFactoryWithEmail(mockHTTP, mockEmail)

	nc := &NotificationChannel{
		Provider: ChannelTypeEmail,
		Data:     ChannelData{Email: "test@example.com"},
	}

	channel, err := factory.CreateChannel(nc)
	if err != nil {
		t.Errorf("CreateChannel() error = %v", err)
		return
	}

	emailChannel, ok := channel.(*EmailChannel)
	if !ok {
		t.Error("expected EmailChannel type")
		return
	}

	if emailChannel.sender != mockEmail {
		t.Error("email sender not injected correctly")
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrConnectionFailed", ErrConnectionFailed},
		{"ErrSendFailed", ErrSendFailed},
		{"ErrInvalidConfiguration", ErrInvalidConfiguration},
		{"ErrChannelDisabled", ErrChannelDisabled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s should not be nil", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s.Error() should not be empty", tt.name)
			}
		})
	}
}
