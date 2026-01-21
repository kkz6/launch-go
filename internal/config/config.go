package config

import (
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/configutil"
	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Cors     CorsConfig
	Queue    QueueConfig
	Billing  BillingConfig
	Git      GitConfig
	Slack    SlackConfig
	Sentry   SentryConfig
}

// Load loads all configuration from environment variables and .env file
func Load() (*Config, error) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("..")

	// Read .env file (optional)
	_ = viper.ReadInConfig()

	// Enable environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Load all configurations using declarative struct tags
	return &Config{
		App:      configutil.Load[AppConfig](),
		Database: configutil.Load[DatabaseConfig](),
		Redis:    configutil.Load[RedisConfig](),
		JWT:      configutil.Load[JWTConfig](),
		Cors:     configutil.Load[CorsConfig](),
		Queue:    configutil.Load[QueueConfig](),
		Billing:  configutil.Load[BillingConfig](),
		Git:      configutil.Load[GitConfig](),
		Slack:    configutil.Load[SlackConfig](),
		Sentry:   configutil.Load[SentryConfig](),
	}, nil
}
