package channels

import (
	"context"
	"fmt"
	"net/http"
)

// SlackChannel handles Slack webhook notifications
type SlackChannel struct {
	channel    *NotificationChannel
	httpClient HTTPClient
}

// slackMessage represents a Slack message payload
type slackMessage struct {
	Text string `json:"text"`
}

// NewSlackChannel creates a new Slack channel
func NewSlackChannel(channel *NotificationChannel, httpClient HTTPClient) *SlackChannel {
	return &SlackChannel{
		channel:    channel,
		httpClient: httpClient,
	}
}

// Send sends a notification via Slack webhook
func (s *SlackChannel) Send(ctx context.Context, notif Notification) error {
	webhookURL := s.channel.GetWebhookURL()
	if webhookURL == "" {
		return ErrInvalidConfiguration
	}

	if s.httpClient == nil {
		return ErrSendFailed
	}

	payload := slackMessage{
		Text: notif.ToSlack(),
	}

	_, statusCode, err := s.httpClient.Post(ctx, webhookURL, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: received status code %d", ErrSendFailed, statusCode)
	}

	return nil
}

// Connect tests the Slack webhook connection
func (s *SlackChannel) Connect(ctx context.Context) error {
	webhookURL := s.channel.GetWebhookURL()
	if webhookURL == "" {
		return ErrInvalidConfiguration
	}

	if s.httpClient == nil {
		return ErrConnectionFailed
	}

	payload := slackMessage{
		Text: "*Connected to Launch*\nThis webhook confirms that you have connected your Slack to Launch.",
	}

	_, statusCode, err := s.httpClient.Post(ctx, webhookURL, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: received status code %d", ErrConnectionFailed, statusCode)
	}

	return nil
}

// GetCreateRules returns the validation rules for creating a Slack channel
func (s *SlackChannel) GetCreateRules() map[string]string {
	return map[string]string{
		"webhook_url":    "required,url",
		"appDeploy":      "boolean",
		"databaseBackup": "boolean",
	}
}

// GetData returns the Slack channel data
func (s *SlackChannel) GetData() ChannelData {
	return s.channel.Data
}
