package channels

import (
	"context"
	"fmt"
	"net/http"
)

// DiscordChannel handles Discord webhook notifications
type DiscordChannel struct {
	channel    *NotificationChannel
	httpClient HTTPClient
}

// discordMessage represents a Discord message payload
type discordMessage struct {
	Content string `json:"content"`
}

// NewDiscordChannel creates a new Discord channel
func NewDiscordChannel(channel *NotificationChannel, httpClient HTTPClient) *DiscordChannel {
	return &DiscordChannel{
		channel:    channel,
		httpClient: httpClient,
	}
}

// Send sends a notification via Discord webhook
func (d *DiscordChannel) Send(ctx context.Context, notif Notification) error {
	webhookURL := d.channel.GetWebhookURL()
	if webhookURL == "" {
		return ErrInvalidConfiguration
	}

	if d.httpClient == nil {
		return ErrSendFailed
	}

	payload := discordMessage{
		Content: notif.ToDiscord(),
	}

	_, statusCode, err := d.httpClient.Post(ctx, webhookURL, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: received status code %d", ErrSendFailed, statusCode)
	}

	return nil
}

// Connect tests the Discord webhook connection
func (d *DiscordChannel) Connect(ctx context.Context) error {
	webhookURL := d.channel.GetWebhookURL()
	if webhookURL == "" {
		return ErrInvalidConfiguration
	}

	if d.httpClient == nil {
		return ErrConnectionFailed
	}

	payload := discordMessage{
		Content: "*Connected to Launch*\nThis webhook confirms that you have connected your Discord to Launch.",
	}

	_, statusCode, err := d.httpClient.Post(ctx, webhookURL, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: received status code %d", ErrConnectionFailed, statusCode)
	}

	return nil
}

// GetCreateRules returns the validation rules for creating a Discord channel
func (d *DiscordChannel) GetCreateRules() map[string]string {
	return map[string]string{
		"webhook_url":    "required,url",
		"appDeploy":      "boolean",
		"databaseBackup": "boolean",
	}
}

// GetData returns the Discord channel data
func (d *DiscordChannel) GetData() ChannelData {
	return d.channel.Data
}
