package config

// SlackConfig holds Slack-related configuration
type SlackConfig struct {
	// AdminWebhookURL is the Slack webhook URL for sending admin alerts
	AdminWebhookURL string `env:"SLACK_ADMIN_WEBHOOK_URL" default:""`
}
