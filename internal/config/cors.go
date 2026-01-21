package config

// CorsConfig holds CORS configuration
type CorsConfig struct {
	AllowedOrigins string `env:"CORS_ALLOWED_ORIGINS" default:"*"`
}
