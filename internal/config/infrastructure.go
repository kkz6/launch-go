package config

import "fmt"

// AppConfig holds application configuration
type AppConfig struct {
	Name        string `env:"APP_NAME" default:"Launch"`
	Environment string `env:"APP_ENV" default:"development"`
	Port        string `env:"APP_PORT" default:"8080"`
	Debug       bool   `env:"APP_DEBUG" default:"true"`
	URL         string `env:"APP_URL" default:"http://localhost:8080"`
	Key         string `env:"APP_KEY" default:""`
	LocalMode   bool   `env:"APP_LOCAL_MODE" default:"false"`
}

// IsLocal returns true if the application is running in local development mode
func (c AppConfig) IsLocal() bool {
	return c.LocalMode || c.Environment == "local" || c.Environment == "development"
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver     string `env:"DB_DRIVER" default:"mysql"`
	Host       string `env:"DB_HOST" default:"localhost"`
	Port       string `env:"DB_PORT" default:"3306"`
	Database   string `env:"DB_DATABASE" default:"launch"`
	Username   string `env:"DB_USERNAME" default:"root"`
	Password   string `env:"DB_PASSWORD" default:""`
	SSLMode    string `env:"DB_SSLMODE" default:"disable"`
	LogQueries bool   `env:"DB_LOG_QUERIES" default:"false"`
}

// DSN returns the database connection string
func (d DatabaseConfig) DSN() string {
	switch d.Driver {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			d.Username, d.Password, d.Host, d.Port, d.Database)
	case "postgres":
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			d.Host, d.Port, d.Username, d.Password, d.Database, d.SSLMode)
	default:
		return ""
	}
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Address  string `env:"REDIS_ADDRESS" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD" default:""`
	DB       int    `env:"REDIS_DB" default:"0"`
}

// QueueConfig holds queue configuration
type QueueConfig struct {
	Concurrency int `env:"QUEUE_CONCURRENCY" default:"10"`
}
