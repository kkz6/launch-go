package client

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configDirName  = "launchctl"
	configFileName = "config.json"
	credsFileName  = "credentials.json"
)

// Config holds CLI configuration
type Config struct {
	APIURL        string `json:"api_url"`
	DefaultTeamID string `json:"default_team_id,omitempty"`
}

// Credentials holds the PAT for API authentication
type Credentials struct {
	Token string `json:"token"`
}

// ConfigDir returns the configuration directory path
func ConfigDir() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, configDirName)
}

// LoadConfig reads the config file
func LoadConfig() (*Config, error) {
	path := filepath.Join(ConfigDir(), configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{APIURL: "https://api.launch.dev"}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.APIURL == "" {
		cfg.APIURL = "https://api.launch.dev"
	}

	return &cfg, nil
}

// SaveConfig writes the config file
func SaveConfig(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, configFileName), data, 0600)
}

// LoadCredentials reads stored credentials
func LoadCredentials() (*Credentials, error) {
	path := filepath.Join(ConfigDir(), credsFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// SaveCredentials writes credentials to disk with secure permissions
func SaveCredentials(creds *Credentials) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, credsFileName), data, 0600)
}

// ClearCredentials removes stored credentials
func ClearCredentials() error {
	path := filepath.Join(ConfigDir(), credsFileName)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
