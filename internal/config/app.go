package config

// AppConfig holds application configuration
type AppConfig struct {
	Name        string
	Environment string
	Port        string
	Debug       bool
	URL         string
	Key         string // Encryption key (base64 encoded, same as Laravel APP_KEY)
	LocalMode   bool   // When true, uses SSH streaming instead of HTTP callbacks for task monitoring
}

// IsLocal returns true if the application is running in local development mode
func (c AppConfig) IsLocal() bool {
	return c.LocalMode || c.Environment == "local" || c.Environment == "development"
}

func loadAppConfig() AppConfig {
	b := NewBuilder()

	return AppConfig{
		Name:        b.String("APP_NAME", "Launch"),
		Environment: b.String("APP_ENV", "development"),
		Port:        b.String("APP_PORT", "8080"),
		Debug:       b.Bool("APP_DEBUG", true),
		URL:         b.String("APP_URL", "http://localhost:8080"),
		Key:         b.String("APP_KEY", ""),
		LocalMode:   b.Bool("APP_LOCAL_MODE", false),
	}
}

// setAppDefaults is kept for backward compatibility but is now a no-op
// since defaults are set inline via the Builder pattern.
func setAppDefaults() {
	// Defaults are now set via Builder in loadAppConfig()
}
