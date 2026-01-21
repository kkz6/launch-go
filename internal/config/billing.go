package config

// BillingConfig holds billing configuration
type BillingConfig struct {
	SubscriptionsEnabled bool   `env:"BILLING_SUBSCRIPTIONS_ENABLED" default:"false"`
	WebhookSecret        string `env:"LEMON_SQUEEZY_SIGNING_SECRET" default:""`
	LemonSqueezy         LemonSqueezyConfig
}

// LemonSqueezyConfig holds Lemon Squeezy provider configuration
type LemonSqueezyConfig struct {
	APIKey  string `env:"LEMON_SQUEEZY_API_KEY" default:""`
	StoreID int    `env:"LEMON_SQUEEZY_STORE" default:"0"`
}
