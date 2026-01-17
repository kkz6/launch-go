package config

import "github.com/spf13/viper"

// SlackConfig holds Slack-related configuration
type SlackConfig struct {
	// AdminWebhookURL is the Slack webhook URL for sending admin alerts
	AdminWebhookURL string
}

// setSlackDefaults sets default values for Slack configuration
func setSlackDefaults() {
	viper.SetDefault("slack.admin_webhook_url", "")
}

// loadSlackConfig loads Slack configuration from environment
func loadSlackConfig() SlackConfig {
	return SlackConfig{
		AdminWebhookURL: viper.GetString("slack.admin_webhook_url"),
	}
}
