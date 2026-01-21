package config

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret     string `env:"JWT_SECRET" default:"change-me-in-production"`
	Expiration int    `env:"JWT_EXPIRATION" default:"72"` // hours
}
