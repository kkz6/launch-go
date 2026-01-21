package channels

import (
	"context"
	"fmt"
	"net/http"
)

// WebhookChannel provides common webhook functionality that can be embedded
// by specific webhook channel implementations (Slack, Discord, etc.)
type WebhookChannel struct {
	httpClient HTTPClient
	webhookURL string
}

// NewWebhookChannel creates a new base webhook channel
func NewWebhookChannel(client HTTPClient, webhookURL string) WebhookChannel {
	return WebhookChannel{
		httpClient: client,
		webhookURL: webhookURL,
	}
}

// ValidateConfig checks if the channel is properly configured
func (w *WebhookChannel) ValidateConfig() error {
	if w.webhookURL == "" {
		return ErrInvalidConfiguration
	}

	if w.httpClient == nil {
		return ErrInvalidConfiguration
	}

	return nil
}

// Post sends a payload to the webhook URL
func (w *WebhookChannel) Post(ctx context.Context, payload any) error {
	if err := w.ValidateConfig(); err != nil {
		return err
	}

	_, statusCode, err := w.httpClient.Post(ctx, w.webhookURL, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: received status code %d", ErrSendFailed, statusCode)
	}

	return nil
}

// GetWebhookURL returns the webhook URL
func (w *WebhookChannel) GetWebhookURL() string {
	return w.webhookURL
}

// SetWebhookURL sets the webhook URL
func (w *WebhookChannel) SetWebhookURL(url string) {
	w.webhookURL = url
}

// GetHTTPClient returns the HTTP client
func (w *WebhookChannel) GetHTTPClient() HTTPClient {
	return w.httpClient
}

// SetHTTPClient sets the HTTP client
func (w *WebhookChannel) SetHTTPClient(client HTTPClient) {
	w.httpClient = client
}

// PostConnect sends a payload to test the webhook connection.
// Returns ErrConnectionFailed instead of ErrSendFailed for failed requests.
func (w *WebhookChannel) PostConnect(ctx context.Context, payload any) error {
	if w.webhookURL == "" {
		return ErrInvalidConfiguration
	}

	if w.httpClient == nil {
		return ErrConnectionFailed
	}

	_, statusCode, err := w.httpClient.Post(ctx, w.webhookURL, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: received status code %d", ErrConnectionFailed, statusCode)
	}

	return nil
}
