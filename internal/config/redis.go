package config

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Address  string `env:"REDIS_ADDRESS" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD" default:""`
	DB       int    `env:"REDIS_DB" default:"0"`
}
