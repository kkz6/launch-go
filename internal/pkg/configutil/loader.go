package configutil

import (
	"reflect"
	"time"

	"github.com/spf13/viper"
)

// Load populates a struct from environment variables using struct tags.
// Supported tags:
//   - `env:"VAR_NAME"` - the environment variable name
//   - `default:"value"` - default value if not set
func Load[T any]() T {
	var cfg T
	loadStruct(reflect.ValueOf(&cfg).Elem())
	return cfg
}

func loadStruct(v reflect.Value) {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Handle embedded structs
		if fieldType.Anonymous && field.Kind() == reflect.Struct {
			loadStruct(field)
			continue
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct && fieldType.Type != reflect.TypeOf(time.Time{}) {
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
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
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
			field.Set(reflect.ValueOf(viper.GetStringSlice(envKey)))
		}
	}
}
