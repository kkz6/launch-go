package config

import "github.com/spf13/viper"

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret     string
	Expiration int // hours
}

func loadJWTConfig() JWTConfig {
	return JWTConfig{
		Secret:     viper.GetString("JWT_SECRET"),
		Expiration: viper.GetInt("JWT_EXPIRATION"),
	}
}

func setJWTDefaults() {
	viper.SetDefault("JWT_SECRET", "change-me-in-production")
	viper.SetDefault("JWT_EXPIRATION", 72)
}
