package channels

import (
	"context"
	"errors"
)

var (
	ErrConnectionFailed     = errors.New("failed to connect to notification channel")
	ErrSendFailed           = errors.New("failed to send notification")
	ErrInvalidConfiguration = errors.New("invalid channel configuration")
	ErrChannelDisabled      = errors.New("notification channel is disabled")
)

// Channel represents a notification channel driver
type Channel interface {
	// Send sends a notification through the channel
	Send(ctx context.Context, notif Notification) error

	// Connect tests the connection to the channel
	Connect(ctx context.Context) error

	// GetCreateRules returns the validation rules for creating the channel
	GetCreateRules() map[string]string

	// GetData returns the channel-specific data
	GetData() ChannelData
}

// Factory creates channel instances based on provider type
type Factory struct {
	httpClient  HTTPClient
	emailSender EmailSender
}

// HTTPClient interface for making HTTP requests
type HTTPClient interface {
	Post(ctx context.Context, url string, body interface{}) ([]byte, int, error)
	Get(ctx context.Context, url string) ([]byte, int, error)
}

// NewFactory creates a new channel factory
func NewFactory(httpClient HTTPClient) *Factory {
	return &Factory{
		httpClient: httpClient,
	}
}

// NewFactoryWithEmail creates a new channel factory with email sender
func NewFactoryWithEmail(httpClient HTTPClient, emailSender EmailSender) *Factory {
	return &Factory{
		httpClient:  httpClient,
		emailSender: emailSender,
	}
}

// CreateChannel creates a channel instance based on provider type
func (f *Factory) CreateChannel(channel *NotificationChannel) (Channel, error) {
	switch channel.Provider {
	case ChannelTypeEmail:
		return NewEmailChannel(channel, f.emailSender), nil
	case ChannelTypeSlack:
		return NewSlackChannel(channel, f.httpClient), nil
	case ChannelTypeDiscord:
		return NewDiscordChannel(channel, f.httpClient), nil
	case ChannelTypeTelegram:
		return NewTelegramChannel(channel, f.httpClient), nil
	default:
		return nil, ErrInvalidConfiguration
	}
}
