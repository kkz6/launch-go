package config

import (
	"os"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

// Loadable interface for configs that can be loaded
type Loadable interface {
	SetDefaults()
	Load()
}

// LoadAll loads multiple configs in order
func LoadAll(configs ...Loadable) {
	for _, c := range configs {
		c.SetDefaults()
		c.Load()
	}
}

// GetEnv gets an environment variable value
func GetEnv(key string) string {
	return viper.GetString(key)
}

// GetEnvOrDefault gets environment variable with default value
func GetEnvOrDefault(key, defaultValue string) string {
	if v := viper.GetString(key); v != "" {
		return v
	}
	return defaultValue
}

// GetEnvOrDefaultDirect gets environment variable directly from OS with default
func GetEnvOrDefaultDirect(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// GetInt gets an integer environment variable
func GetInt(key string) int {
	return viper.GetInt(key)
}

// GetIntOrDefault gets int env var with default
func GetIntOrDefault(key string, defaultValue int) int {
	if viper.IsSet(key) {
		return viper.GetInt(key)
	}
	return defaultValue
}

// GetIntOrDefaultDirect gets int env var directly from OS with default
func GetIntOrDefaultDirect(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

// GetBool gets a boolean environment variable
func GetBool(key string) bool {
	return viper.GetBool(key)
}

// GetBoolOrDefault gets bool env var with default
func GetBoolOrDefault(key string, defaultValue bool) bool {
	if viper.IsSet(key) {
		return viper.GetBool(key)
	}
	return defaultValue
}

// GetBoolOrDefaultDirect gets bool env var directly from OS with default
func GetBoolOrDefaultDirect(key string, defaultValue bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultValue
}

// GetDuration gets a duration environment variable
func GetDuration(key string) time.Duration {
	return viper.GetDuration(key)
}

// GetDurationOrDefault gets duration env var with default
func GetDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if viper.IsSet(key) {
		return viper.GetDuration(key)
	}
	return defaultValue
}

// GetDurationOrDefaultDirect gets duration env var directly from OS with default
func GetDurationOrDefaultDirect(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}

// GetFloat64 gets a float64 environment variable
func GetFloat64(key string) float64 {
	return viper.GetFloat64(key)
}

// GetFloat64OrDefault gets float64 env var with default
func GetFloat64OrDefault(key string, defaultValue float64) float64 {
	if viper.IsSet(key) {
		return viper.GetFloat64(key)
	}
	return defaultValue
}

// MustGetEnv gets an environment variable or panics if not set
func MustGetEnv(key string) string {
	v := viper.GetString(key)
	if v == "" {
		panic("required environment variable not set: " + key)
	}
	return v
}

// IsSet checks if a configuration key is set
func IsSet(key string) bool {
	return viper.IsSet(key)
}

// SetDefault sets a default value for a configuration key
func SetDefault(key string, value interface{}) {
	viper.SetDefault(key, value)
}
