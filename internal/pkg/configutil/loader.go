package configutil

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Validator interface for config validation
type Validator interface {
	Validate() error
}

// Load populates a struct from environment variables using struct tags.
// This is the generic version that returns a new instance.
// Supported tags:
//   - `env:"VAR_NAME"` - the environment variable name
//   - `default:"value"` - default value if not set
func Load[T any]() T {
	var cfg T
	loadStruct(reflect.ValueOf(&cfg).Elem())
	return cfg
}

// LoadInto populates a config struct from viper using struct tags.
// Tag format: `env:"ENV_VAR_NAME" default:"default_value"`
//
// Supported field types: string, bool, int, int64, float64, time.Duration, []string
//
// Example:
//
//	type AppConfig struct {
//	    Name string `env:"APP_NAME" default:"Launch"`
//	    Port int    `env:"APP_PORT" default:"8080"`
//	}
//
//	var cfg AppConfig
//	if err := configutil.LoadInto(&cfg); err != nil {
//	    log.Fatal(err)
//	}
func LoadInto(cfg any) error {
	v := reflect.ValueOf(cfg)

	if v.Kind() != reflect.Ptr {
		return errors.New("configutil.LoadInto: cfg must be a pointer")
	}

	if v.IsNil() {
		return errors.New("configutil.LoadInto: cfg cannot be nil")
	}

	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return errors.New("configutil.LoadInto: cfg must be a pointer to a struct")
	}

	loadStruct(elem)

	return nil
}

// LoadAndValidate loads config into a pointer and calls Validate() if implemented
func LoadAndValidate(cfg any) error {
	if err := LoadInto(cfg); err != nil {
		return err
	}

	if validator, ok := cfg.(Validator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// MustLoadInto loads config and panics on error
func MustLoadInto(cfg any) {
	if err := LoadInto(cfg); err != nil {
		panic(err)
	}
}

// MustLoadAndValidate loads and validates config, panicking on error
func MustLoadAndValidate(cfg any) {
	if err := LoadAndValidate(cfg); err != nil {
		panic(err)
	}
}

func loadStruct(v reflect.Value) {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Handle embedded structs
		if fieldType.Anonymous && field.Kind() == reflect.Struct {
			loadStruct(field)

			continue
		}

		// Handle nested structs (but not time.Duration which is int64)
		if field.Kind() == reflect.Struct && fieldType.Type != reflect.TypeFor[time.Time]() {
			loadStruct(field)

			continue
		}

		envKey := fieldType.Tag.Get("env")
		if envKey == "" {
			continue
		}

		defaultVal := fieldType.Tag.Get("default")

		// Set default in viper
		if defaultVal != "" {
			viper.SetDefault(envKey, defaultVal)
		}

		// Get value and set field
		setFieldValue(field, envKey)
	}
}

func setFieldValue(field reflect.Value, envKey string) {
	if !field.CanSet() {
		return
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(viper.GetString(envKey))

	case reflect.Int, reflect.Int64:
		if field.Type() == reflect.TypeFor[time.Duration]() {
			field.Set(reflect.ValueOf(viper.GetDuration(envKey)))
		} else {
			field.SetInt(viper.GetInt64(envKey))
		}

	case reflect.Bool:
		field.SetBool(viper.GetBool(envKey))

	case reflect.Float64:
		field.SetFloat(viper.GetFloat64(envKey))

	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			// viper.GetStringSlice doesn't split comma-separated env vars,
			// so we handle it manually
			val := viper.GetString(envKey)
			if val != "" {
				parts := strings.Split(val, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				field.Set(reflect.ValueOf(parts))
			}
		}
	}
}
