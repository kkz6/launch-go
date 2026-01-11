package config

import "github.com/spf13/viper"

// AppConfig holds application configuration
type AppConfig struct {
	Name        string
	Environment string
	Port        string
	Debug       bool
	URL         string
	Key         string // Encryption key (base64 encoded, same as Laravel APP_KEY)
}

func loadAppConfig() AppConfig {
	return AppConfig{
		Name:        viper.GetString("APP_NAME"),
		Environment: viper.GetString("APP_ENV"),
		Port:        viper.GetString("APP_PORT"),
		Debug:       viper.GetBool("APP_DEBUG"),
		URL:         viper.GetString("APP_URL"),
		Key:         viper.GetString("APP_KEY"),
	}
}

func setAppDefaults() {
	viper.SetDefault("APP_NAME", "Launch")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("APP_DEBUG", true)
	viper.SetDefault("APP_URL", "http://localhost:8080")
}
