package config

import (
	"strings"

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

	// Set defaults from all config files
	setDefaults()

	// Load all configurations
	return &Config{
		App:      loadAppConfig(),
		Database: loadDatabaseConfig(),
		Redis:    loadRedisConfig(),
		JWT:      loadJWTConfig(),
		Cors:     loadCorsConfig(),
		Queue:    loadQueueConfig(),
		Billing:  loadBillingConfig(),
		Git:      loadGitConfig(),
		Slack:    loadSlackConfig(),
	}, nil
}

// setDefaults sets all default values
func setDefaults() {
	setAppDefaults()
	setDatabaseDefaults()
	setRedisDefaults()
	setJWTDefaults()
	setCorsDefaults()
	setQueueDefaults()
	setBillingDefaults()
	setGitDefaults()
	setSlackDefaults()
}
