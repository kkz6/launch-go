package channels

import (
	"context"
)

// SlackChannel handles Slack webhook notifications
type SlackChannel struct {
	WebhookChannel
	channel *NotificationChannel
}

// slackMessage represents a Slack message payload
type slackMessage struct {
	Text string `json:"text"`
}

// NewSlackChannel creates a new Slack channel
func NewSlackChannel(channel *NotificationChannel, httpClient HTTPClient) *SlackChannel {
	return &SlackChannel{
		WebhookChannel: NewWebhookChannel(httpClient, channel.GetWebhookURL()),
		channel:        channel,
	}
}

// Send sends a notification via Slack webhook
func (s *SlackChannel) Send(ctx context.Context, notif Notification) error {
	return s.Post(ctx, slackMessage{Text: notif.ToSlack()})
}

// Connect tests the Slack webhook connection
func (s *SlackChannel) Connect(ctx context.Context) error {
	return s.PostConnect(ctx, slackMessage{
		Text: "*Connected to launchctl*\nThis webhook confirms that you have connected your Slack to launchctl.",
	})
}

// GetCreateRules returns the validation rules for creating a Slack channel
func (s *SlackChannel) GetCreateRules() map[string]string {
	return MergeValidationRules(map[string]string{
		"webhook_url": "required,url",
	})
}

// GetData returns the Slack channel data
func (s *SlackChannel) GetData() ChannelData {
	return s.channel.Data
}
