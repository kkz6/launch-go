package config

import (
	"time"

	"github.com/spf13/viper"
)

// Builder provides a fluent interface for reading configuration values with defaults.
// It combines viper.SetDefault and viper.Get into single method calls,
// ensuring defaults are always registered before reading values.
type Builder struct{}

// NewBuilder creates a new configuration builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// String returns a string configuration value.
// It sets the default value and then retrieves the current value.
func (b *Builder) String(key string, defaultValue string) string {
	viper.SetDefault(key, defaultValue)

	return viper.GetString(key)
}

// Int returns an integer configuration value.
// It sets the default value and then retrieves the current value.
func (b *Builder) Int(key string, defaultValue int) int {
	viper.SetDefault(key, defaultValue)

	return viper.GetInt(key)
}

// Bool returns a boolean configuration value.
// It sets the default value and then retrieves the current value.
func (b *Builder) Bool(key string, defaultValue bool) bool {
	viper.SetDefault(key, defaultValue)

	return viper.GetBool(key)
}

// Duration returns a time.Duration configuration value.
// It sets the default value and then retrieves the current value.
// The value can be specified as a duration string (e.g., "30s", "5m", "1h").
func (b *Builder) Duration(key string, defaultValue time.Duration) time.Duration {
	viper.SetDefault(key, defaultValue)

	return viper.GetDuration(key)
}

// StringSlice returns a string slice configuration value.
// It sets the default value and then retrieves the current value.
func (b *Builder) StringSlice(key string, defaultValue []string) []string {
	viper.SetDefault(key, defaultValue)

	return viper.GetStringSlice(key)
}

// Float64 returns a float64 configuration value.
// It sets the default value and then retrieves the current value.
func (b *Builder) Float64(key string, defaultValue float64) float64 {
	viper.SetDefault(key, defaultValue)

	return viper.GetFloat64(key)
}

// Int64 returns an int64 configuration value.
// It sets the default value and then retrieves the current value.
func (b *Builder) Int64(key string, defaultValue int64) int64 {
	viper.SetDefault(key, defaultValue)

	return viper.GetInt64(key)
}
