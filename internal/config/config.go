package config

import (
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/config"
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
	Mail     MailConfig
	Core     CoreConfig
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
		App:      config.Load[AppConfig](),
		Database: config.Load[DatabaseConfig](),
		Redis:    config.Load[RedisConfig](),
		JWT:      config.Load[JWTConfig](),
		Cors:     config.Load[CorsConfig](),
		Queue:    config.Load[QueueConfig](),
		Billing:  config.Load[BillingConfig](),
		Git:      config.Load[GitConfig](),
		Slack:    config.Load[SlackConfig](),
		Sentry:   config.Load[SentryConfig](),
		Mail:     config.Load[MailConfig](),
		Core:     *GetCoreConfig(),
	}, nil
}
