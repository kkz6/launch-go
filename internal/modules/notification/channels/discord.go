package channels

import (
	"context"
)

// DiscordChannel handles Discord webhook notifications
type DiscordChannel struct {
	WebhookChannel
	channel *NotificationChannel
}

// discordMessage represents a Discord message payload
type discordMessage struct {
	Content string `json:"content"`
}

// NewDiscordChannel creates a new Discord channel
func NewDiscordChannel(channel *NotificationChannel, httpClient HTTPClient) *DiscordChannel {
	return &DiscordChannel{
		WebhookChannel: NewWebhookChannel(httpClient, channel.GetWebhookURL()),
		channel:        channel,
	}
}

// Send sends a notification via Discord webhook
func (d *DiscordChannel) Send(ctx context.Context, notif Notification) error {
	return d.Post(ctx, discordMessage{Content: notif.ToDiscord()})
}

// Connect tests the Discord webhook connection
func (d *DiscordChannel) Connect(ctx context.Context) error {
	return d.PostConnect(ctx, discordMessage{
		Content: "*Connected to Launch*\nThis webhook confirms that you have connected your Discord to Launch.",
	})
}

// GetCreateRules returns the validation rules for creating a Discord channel
func (d *DiscordChannel) GetCreateRules() map[string]string {
	return MergeValidationRules(map[string]string{
		"webhook_url": "required,url",
	})
}

// GetData returns the Discord channel data
func (d *DiscordChannel) GetData() ChannelData {
	return d.channel.Data
}
