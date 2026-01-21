package config

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
