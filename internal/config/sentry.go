package config

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
