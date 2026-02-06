package templates

import "sync"

// Config holds the email template configuration
type Config struct {
	AppName string
	AppURL  string
}

var (
	globalConfig Config
	configMu     sync.RWMutex
)

// Initialize sets the global configuration for email templates
func Initialize(appName, appURL string) {
	configMu.Lock()
	defer configMu.Unlock()
	globalConfig = Config{
		AppName: appName,
		AppURL:  appURL,
	}
}

// GetConfig returns the current configuration
func GetConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

// NewEmail creates a new email builder with the global configuration
func NewEmail() *EmailBuilder {
	cfg := GetConfig()
	return NewEmailBuilder(cfg.AppName, cfg.AppURL)
}
