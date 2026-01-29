package config

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret     string `env:"JWT_SECRET" default:"change-me-in-production"`
	Expiration int    `env:"JWT_EXPIRATION" default:"72"` // hours
}

// CorsConfig holds CORS configuration
type CorsConfig struct {
	AllowedOrigins string `env:"CORS_ALLOWED_ORIGINS" default:"*"`
}

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

// GitConfig holds git provider configuration
type GitConfig struct {
	GitHub    GitHubConfig
	GitLab    GitLabConfig
	Bitbucket BitbucketConfig
}

// GitHubConfig holds GitHub-specific configuration
type GitHubConfig struct {
	AppID         string `env:"GITHUB_APP_ID" default:""`
	PrivateKey    string `env:"GITHUB_PRIVATE_KEY" default:""`
	WebhookSecret string `env:"GITHUB_WEBHOOK_SECRET" default:""`
	AppSlug       string `env:"GITHUB_APP_SLUG" default:""`
}

// GitLabConfig holds GitLab-specific configuration
type GitLabConfig struct {
	ClientID      string `env:"GITLAB_CLIENT_ID" default:""`
	ClientSecret  string `env:"GITLAB_CLIENT_SECRET" default:""`
	WebhookSecret string `env:"GITLAB_WEBHOOK_SECRET" default:""`
}

// BitbucketConfig holds Bitbucket-specific configuration
type BitbucketConfig struct {
	ClientID      string `env:"BITBUCKET_CLIENT_ID" default:""`
	ClientSecret  string `env:"BITBUCKET_CLIENT_SECRET" default:""`
	WebhookSecret string `env:"BITBUCKET_WEBHOOK_SECRET" default:""`
}

// SlackConfig holds Slack-related configuration
type SlackConfig struct {
	// AdminWebhookURL is the Slack webhook URL for sending admin alerts
	AdminWebhookURL string `env:"SLACK_ADMIN_WEBHOOK_URL" default:""`
}

// SentryConfig holds Sentry error tracking configuration
type SentryConfig struct {
	DSN              string  `env:"SENTRY_DSN" default:""`
	Enabled          bool    `env:"SENTRY_ENABLED" default:"false"`
	Environment      string  `env:"SENTRY_ENVIRONMENT" default:""`
	Debug            bool    `env:"SENTRY_DEBUG" default:"false"`
	SampleRate       float64 `env:"SENTRY_SAMPLE_RATE" default:"1.0"`
	TracesSampleRate float64 `env:"SENTRY_TRACES_SAMPLE_RATE" default:"0.1"`
}

// IsEnabled returns true if Sentry should be enabled
// Sentry is only enabled when DSN is set and Enabled flag is true
func (c SentryConfig) IsEnabled() bool {
	return c.Enabled && c.DSN != ""
}

// MailConfig holds email sending configuration
type MailConfig struct {
	Driver      string `env:"MAIL_DRIVER" default:"resend"`
	FromAddress string `env:"MAIL_FROM_ADDRESS" default:"noreply@example.com"`
	FromName    string `env:"MAIL_FROM_NAME" default:"Launch"`
	ResendKey   string `env:"RESEND_API_KEY" default:""`
	SMTPHost    string `env:"SMTP_HOST" default:""`
	SMTPPort    int    `env:"SMTP_PORT" default:"587"`
	SMTPUser    string `env:"SMTP_USERNAME" default:""`
	SMTPPass    string `env:"SMTP_PASSWORD" default:""`
}
