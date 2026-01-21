package config

import "fmt"

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
