package config

import "github.com/spf13/viper"

// SentryConfig holds Sentry error tracking configuration
type SentryConfig struct {
	DSN              string
	Enabled          bool
	Environment      string
	Debug            bool
	SampleRate       float64
	TracesSampleRate float64
}

// IsEnabled returns true if Sentry should be enabled
// Sentry is only enabled when DSN is set and environment is production
func (c SentryConfig) IsEnabled() bool {
	return c.Enabled && c.DSN != ""
}

func loadSentryConfig() SentryConfig {
	env := viper.GetString("APP_ENV")

	// Only enable Sentry in production by default
	enabled := viper.GetBool("SENTRY_ENABLED")
	if !viper.IsSet("SENTRY_ENABLED") {
		enabled = env == "production"
	}

	return SentryConfig{
		DSN:              viper.GetString("SENTRY_DSN"),
		Enabled:          enabled,
		Environment:      env,
		Debug:            viper.GetBool("SENTRY_DEBUG"),
		SampleRate:       viper.GetFloat64("SENTRY_SAMPLE_RATE"),
		TracesSampleRate: viper.GetFloat64("SENTRY_TRACES_SAMPLE_RATE"),
	}
}

func setSentryDefaults() {
	viper.SetDefault("SENTRY_DSN", "")
	viper.SetDefault("SENTRY_DEBUG", false)
	viper.SetDefault("SENTRY_SAMPLE_RATE", 1.0)
	viper.SetDefault("SENTRY_TRACES_SAMPLE_RATE", 0.1)
}
