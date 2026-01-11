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
	LocalMode   bool   // When true, uses SSH streaming instead of HTTP callbacks for task monitoring
}

// IsLocal returns true if the application is running in local development mode
func (c AppConfig) IsLocal() bool {
	return c.LocalMode || c.Environment == "local" || c.Environment == "development"
}

func loadAppConfig() AppConfig {
	return AppConfig{
		Name:        viper.GetString("APP_NAME"),
		Environment: viper.GetString("APP_ENV"),
		Port:        viper.GetString("APP_PORT"),
		Debug:       viper.GetBool("APP_DEBUG"),
		URL:         viper.GetString("APP_URL"),
		Key:         viper.GetString("APP_KEY"),
		LocalMode:   viper.GetBool("APP_LOCAL_MODE"),
	}
}

func setAppDefaults() {
	viper.SetDefault("APP_NAME", "Launch")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("APP_DEBUG", true)
	viper.SetDefault("APP_URL", "http://localhost:8080")
	viper.SetDefault("APP_LOCAL_MODE", false)
}
