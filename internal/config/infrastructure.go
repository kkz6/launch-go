package config

import (
	"fmt"
	"net/url"
	"strings"
)

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
	URL        string `env:"DB_URL" default:""`
	Driver     string `env:"DB_DRIVER" default:"mysql"`
	Host       string `env:"DB_HOST" default:"localhost"`
	Port       string `env:"DB_PORT" default:"3306"`
	Database   string `env:"DB_DATABASE" default:"launch"`
	Username   string `env:"DB_USERNAME" default:"root"`
	Password   string `env:"DB_PASSWORD" default:""`
	SSLMode    string `env:"DB_SSLMODE" default:"disable"`
	LogQueries bool   `env:"DB_LOG_QUERIES" default:"false"`
}

// ResolveDriver returns the effective database driver, inferring from URL if set.
func (d DatabaseConfig) ResolveDriver() string {
	if d.URL != "" {
		if strings.HasPrefix(d.URL, "postgres://") || strings.HasPrefix(d.URL, "postgresql://") {
			return "postgres"
		}

		if strings.HasPrefix(d.URL, "mysql://") {
			return "mysql"
		}
	}

	return d.Driver
}

// DSN returns the database connection string.
// If DB_URL is set, it is parsed and converted to the driver-specific format.
// Otherwise, the DSN is built from individual fields.
func (d DatabaseConfig) DSN() string {
	if d.URL != "" {
		driver := d.ResolveDriver()

		if driver == "mysql" {
			return mysqlURLToDSN(d.URL)
		}

		// PostgreSQL accepts the URL as-is
		return d.URL
	}

	switch d.Driver {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=True&loc=Local",
			d.Username, d.Password, d.Host, d.Port, d.Database)
	case "postgres":
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			d.Host, d.Port, d.Username, d.Password, d.Database, d.SSLMode)
	default:
		return ""
	}
}

// mysqlURLToDSN converts a mysql:// URL to Go MySQL driver DSN format.
// Input:  mysql://user:pass@host:port/dbname?params
// Output: user:pass@tcp(host:port)/dbname?params
func mysqlURLToDSN(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimPrefix(rawURL, "mysql://")
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "3306"
	}

	dbName := strings.TrimPrefix(u.Path, "/")

	password, _ := u.User.Password()
	userInfo := u.User.Username()
	if password != "" {
		userInfo += ":" + password
	}

	dsn := fmt.Sprintf("%s@tcp(%s:%s)/%s", userInfo, host, port, dbName)

	query := u.Query()
	if len(query) == 0 {
		dsn += "?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=True&loc=Local"
	} else {
		if query.Get("collation") == "" {
			query.Set("collation", "utf8mb4_unicode_ci")
		}
		if query.Get("parseTime") == "" {
			query.Set("parseTime", "True")
		}
		dsn += "?" + query.Encode()
	}

	return dsn
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
