package config

import (
	"github.com/spf13/viper"
)

// BillingConfig holds billing configuration
type BillingConfig struct {
	SubscriptionsEnabled bool
	WebhookSecret        string
	LemonSqueezy         LemonSqueezyConfig
}

// LemonSqueezyConfig holds Lemon Squeezy provider configuration
type LemonSqueezyConfig struct {
	APIKey  string
	StoreID int
}

func loadBillingConfig() BillingConfig {
	return BillingConfig{
		SubscriptionsEnabled: viper.GetBool("BILLING_SUBSCRIPTIONS_ENABLED"),
		WebhookSecret:        viper.GetString("LEMON_SQUEEZY_SIGNING_SECRET"),
		LemonSqueezy: LemonSqueezyConfig{
			APIKey:  viper.GetString("LEMON_SQUEEZY_API_KEY"),
			StoreID: viper.GetInt("LEMON_SQUEEZY_STORE"),
		},
	}
}

func setBillingDefaults() {
	viper.SetDefault("BILLING_SUBSCRIPTIONS_ENABLED", false)
	viper.SetDefault("LEMON_SQUEEZY_SIGNING_SECRET", "")
	viper.SetDefault("LEMON_SQUEEZY_API_KEY", "")
	viper.SetDefault("LEMON_SQUEEZY_STORE", 0)
}
