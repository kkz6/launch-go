package config

import (
	"fmt"
	"net/url"
	"strings"
)

// AppConfig holds application configuration
type AppConfig struct {
	Name        string `env:"APP_NAME" default:"launchctl"`
	Environment string `env:"APP_ENV" default:"development"`
	Port        string `env:"APP_PORT" default:"8080"`
	Debug       bool   `env:"APP_DEBUG" default:"true"`
	// URL is the API's own public URL (e.g. https://api.example.com).
	// Used for API self-reference: signed-URL bases, webhook callback URLs
	// emitted by this app to itself, server-callback URLs sent to
	// provisioned servers, etc.
	URL string `env:"APP_URL" default:"http://localhost:8080"`
	// FrontendURL is the public URL of the user-facing frontend
	// (e.g. https://example.com — typically Nuxt/SPA, on a DIFFERENT
	// host than the API). Used in EMAIL LINKS and OAuth/payment-flow
	// redirects so users land on the UI, not the API.
	// Falls back to URL if unset (single-host deployments).
	FrontendURL string `env:"APP_FRONTEND_URL" default:""`
	Key         string `env:"APP_KEY" default:""`
	LocalMode   bool   `env:"APP_LOCAL_MODE" default:"false"`
}

// IsLocal returns true if the application is running in local development mode
func (c AppConfig) IsLocal() bool {
	return c.LocalMode || c.Environment == "local" || c.Environment == "development"
}

// Frontend returns the user-facing frontend URL. Falls back to URL when
// APP_FRONTEND_URL is unset (single-host dev / legacy deployments).
// Always use this — not c.URL — for links sent to users (emails,
// OAuth/billing redirects).
func (c AppConfig) Frontend() string {
	if c.FrontendURL != "" {
		return c.FrontendURL
	}
	return c.URL
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
		// Use single-quoted values for password/sslmode so libpq's
		// keyword-value parser handles empty/special-character values
		// correctly. Without quotes, `password= dbname=...` is parsed
		// as password being the rest of the string.
		password := strings.ReplaceAll(d.Password, "'", "\\'")
		return fmt.Sprintf("host=%s port=%s user=%s password='%s' dbname=%s sslmode=%s",
			d.Host, d.Port, d.Username, password, d.Database, d.SSLMode)
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

// PasskeyConfig holds WebAuthn/Passkey configuration
type PasskeyConfig struct {
	RPName     string `env:"PASSKEY_RP_NAME" default:""`
	RPID       string `env:"PASSKEY_RP_ID" default:""`
	RPOrigin   string `env:"PASSKEY_RP_ORIGIN" default:""`
	Timeout    int    `env:"PASSKEY_TIMEOUT" default:"60000"`
	MaxPerUser int    `env:"PASSKEY_MAX_PER_USER" default:"10"`
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

// ProxyConfig holds reverse proxy configuration
type ProxyConfig struct {
	TrustedProxies string `env:"TRUSTED_PROXIES" default:""`
	ProxyHeader    string `env:"PROXY_HEADER" default:"X-Forwarded-For"`
}
