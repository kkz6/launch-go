package config

import "github.com/spf13/viper"

// CorsConfig holds CORS configuration
type CorsConfig struct {
	AllowedOrigins string
}

func loadCorsConfig() CorsConfig {
	return CorsConfig{
		AllowedOrigins: viper.GetString("CORS_ALLOWED_ORIGINS"),
	}
}

func setCorsDefaults() {
	viper.SetDefault("CORS_ALLOWED_ORIGINS", "*")
}
