package configutil

import (
	"errors"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// resetViper clears viper state between tests
func resetViper() {
	viper.Reset()
	viper.AutomaticEnv()
}

func TestLoad_BasicTypes(t *testing.T) {
	resetViper()

	t.Setenv("TEST_STRING", "hello")
	t.Setenv("TEST_INT", "42")
	t.Setenv("TEST_INT64", "9223372036854775807")
	t.Setenv("TEST_BOOL", "true")
	t.Setenv("TEST_FLOAT64", "3.14")

	type Config struct {
		String  string  `env:"TEST_STRING"`
		Int     int     `env:"TEST_INT"`
		Int64   int64   `env:"TEST_INT64"`
		Bool    bool    `env:"TEST_BOOL"`
		Float64 float64 `env:"TEST_FLOAT64"`
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.String != "hello" {
		t.Errorf("String = %v, want %v", cfg.String, "hello")
	}

	if cfg.Int != 42 {
		t.Errorf("Int = %v, want %v", cfg.Int, 42)
	}

	if cfg.Int64 != 9223372036854775807 {
		t.Errorf("Int64 = %v, want %v", cfg.Int64, int64(9223372036854775807))
	}

	if !cfg.Bool {
		t.Errorf("Bool = %v, want %v", cfg.Bool, true)
	}

	if cfg.Float64 != 3.14 {
		t.Errorf("Float64 = %v, want %v", cfg.Float64, 3.14)
	}
}

func TestLoad_Duration(t *testing.T) {
	resetViper()

	t.Setenv("TEST_DURATION", "5m30s")

	type Config struct {
		Duration time.Duration `env:"TEST_DURATION"`
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	expected := 5*time.Minute + 30*time.Second
	if cfg.Duration != expected {
		t.Errorf("Duration = %v, want %v", cfg.Duration, expected)
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	resetViper()

	type Config struct {
		Name string `env:"TEST_NAME" default:"default_name"`
		Port int    `env:"TEST_PORT" default:"8080"`
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.Name != "default_name" {
		t.Errorf("Name = %v, want %v", cfg.Name, "default_name")
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %v, want %v", cfg.Port, 8080)
	}
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	resetViper()

	t.Setenv("TEST_NAME", "custom_name")
	t.Setenv("TEST_PORT", "9090")

	type Config struct {
		Name string `env:"TEST_NAME" default:"default_name"`
		Port int    `env:"TEST_PORT" default:"8080"`
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.Name != "custom_name" {
		t.Errorf("Name = %v, want %v", cfg.Name, "custom_name")
	}

	if cfg.Port != 9090 {
		t.Errorf("Port = %v, want %v", cfg.Port, 9090)
	}
}

func TestLoad_NestedStructs(t *testing.T) {
	resetViper()

	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("CACHE_TTL", "10m")
	t.Setenv("APP_NAME", "myapp")

	type Database struct {
		Host string `env:"DB_HOST"`
		Port int    `env:"DB_PORT"`
	}

	type Cache struct {
		TTL time.Duration `env:"CACHE_TTL"`
	}

	type Config struct {
		Name     string `env:"APP_NAME"`
		Database Database
		Cache    Cache
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.Name != "myapp" {
		t.Errorf("Name = %v, want %v", cfg.Name, "myapp")
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %v, want %v", cfg.Database.Host, "localhost")
	}

	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %v, want %v", cfg.Database.Port, 5432)
	}

	if cfg.Cache.TTL != 10*time.Minute {
		t.Errorf("Cache.TTL = %v, want %v", cfg.Cache.TTL, 10*time.Minute)
	}
}

func TestLoad_EmbeddedStructs(t *testing.T) {
	resetViper()

	t.Setenv("EMBED_NAME", "embedded")
	t.Setenv("EMBED_VALUE", "123")
	t.Setenv("OUTER_FIELD", "outer")

	type Embedded struct {
		Name  string `env:"EMBED_NAME"`
		Value int    `env:"EMBED_VALUE"`
	}

	type Config struct {
		Embedded
		OuterField string `env:"OUTER_FIELD"`
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.Name != "embedded" {
		t.Errorf("Embedded.Name = %v, want %v", cfg.Name, "embedded")
	}

	if cfg.Value != 123 {
		t.Errorf("Embedded.Value = %v, want %v", cfg.Value, 123)
	}

	if cfg.OuterField != "outer" {
		t.Errorf("OuterField = %v, want %v", cfg.OuterField, "outer")
	}
}

func TestLoad_NoEnvTag(t *testing.T) {
	resetViper()

	t.Setenv("SOME_VALUE", "ignored")

	type Config struct {
		NoTag     string
		WithTag   string `env:"SOME_VALUE"`
		EmptyTag  string `env:""`
		OtherTags string `json:"other"`
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.NoTag != "" {
		t.Errorf("NoTag = %v, want empty", cfg.NoTag)
	}

	if cfg.WithTag != "ignored" {
		t.Errorf("WithTag = %v, want %v", cfg.WithTag, "ignored")
	}
}

func TestLoad_ErrorNonPointer(t *testing.T) {
	resetViper()

	type Config struct {
		Name string `env:"NAME"`
	}

	cfg := Config{}
	err := LoadInto(cfg) // Not a pointer

	if err == nil {
		t.Fatal("LoadInto() expected error for non-pointer, got nil")
	}

	if err.Error() != "configutil.LoadInto: cfg must be a pointer" {
		t.Errorf("LoadInto() error = %v, want 'configutil.LoadInto: cfg must be a pointer'", err)
	}
}

func TestLoad_ErrorNilPointer(t *testing.T) {
	resetViper()

	type Config struct {
		Name string `env:"NAME"`
	}

	var cfg *Config
	err := LoadInto(cfg) // Nil pointer

	if err == nil {
		t.Fatal("LoadInto() expected error for nil pointer, got nil")
	}

	if err.Error() != "configutil.LoadInto: cfg cannot be nil" {
		t.Errorf("LoadInto() error = %v, want 'configutil.LoadInto: cfg cannot be nil'", err)
	}
}

func TestLoad_ErrorNotStruct(t *testing.T) {
	resetViper()

	var notAStruct string
	err := LoadInto(&notAStruct)

	if err == nil {
		t.Fatal("LoadInto() expected error for non-struct, got nil")
	}

	if err.Error() != "configutil.LoadInto: cfg must be a pointer to a struct" {
		t.Errorf("LoadInto() error = %v, want 'configutil.LoadInto: cfg must be a pointer to a struct'", err)
	}
}

func TestLoad_StringSlice(t *testing.T) {
	resetViper()

	t.Setenv("TEST_HOSTS", "host1,host2,host3")

	type Config struct {
		Hosts []string `env:"TEST_HOSTS"`
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	expected := []string{"host1", "host2", "host3"}
	if len(cfg.Hosts) != len(expected) {
		t.Errorf("Hosts length = %v, want %v", len(cfg.Hosts), len(expected))
	}

	for i, host := range cfg.Hosts {
		if host != expected[i] {
			t.Errorf("Hosts[%d] = %v, want %v", i, host, expected[i])
		}
	}
}

// validatableConfig is a config that implements Validator
type validatableConfig struct {
	Name string `env:"VALIDATE_NAME"`
}

func (c validatableConfig) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func TestLoadAndValidate_Success(t *testing.T) {
	resetViper()

	t.Setenv("VALIDATE_NAME", "valid")

	var cfg validatableConfig
	err := LoadAndValidate(&cfg)

	if err != nil {
		t.Fatalf("LoadAndValidate() error = %v", err)
	}

	if cfg.Name != "valid" {
		t.Errorf("Name = %v, want %v", cfg.Name, "valid")
	}
}

func TestLoadAndValidate_ValidationFails(t *testing.T) {
	resetViper()

	// Name not set, should fail validation
	var cfg validatableConfig
	err := LoadAndValidate(&cfg)

	if err == nil {
		t.Fatal("LoadAndValidate() expected validation error, got nil")
	}

	if err.Error() != "name is required" {
		t.Errorf("LoadAndValidate() error = %v, want 'name is required'", err)
	}
}

func TestLoadAndValidate_NoValidator(t *testing.T) {
	resetViper()

	t.Setenv("NO_VALIDATE_NAME", "test")

	type Config struct {
		Name string `env:"NO_VALIDATE_NAME"`
	}

	var cfg Config
	err := LoadAndValidate(&cfg)

	if err != nil {
		t.Fatalf("LoadAndValidate() error = %v", err)
	}

	if cfg.Name != "test" {
		t.Errorf("Name = %v, want %v", cfg.Name, "test")
	}
}

func TestMustLoadInto_Success(t *testing.T) {
	resetViper()

	t.Setenv("MUST_LOAD_NAME", "test")

	type Config struct {
		Name string `env:"MUST_LOAD_NAME"`
	}

	var cfg Config

	// Should not panic
	MustLoadInto(&cfg)

	if cfg.Name != "test" {
		t.Errorf("Name = %v, want %v", cfg.Name, "test")
	}
}

func TestMustLoadInto_Panics(t *testing.T) {
	resetViper()

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustLoadInto() expected panic, got none")
		}
	}()

	type Config struct {
		Name string `env:"NAME"`
	}

	var cfg Config
	MustLoadInto(cfg) // Not a pointer, should panic
}

func TestMustLoadAndValidate_Success(t *testing.T) {
	resetViper()

	t.Setenv("VALIDATE_NAME", "test")

	var cfg validatableConfig

	// Should not panic
	MustLoadAndValidate(&cfg)

	if cfg.Name != "test" {
		t.Errorf("Name = %v, want %v", cfg.Name, "test")
	}
}

func TestMustLoadAndValidate_PanicsOnValidation(t *testing.T) {
	resetViper()

	// Name not set, should fail validation and panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustLoadAndValidate() expected panic, got none")
		}
	}()

	var cfg validatableConfig
	MustLoadAndValidate(&cfg)
}

func TestLoad_BoolFalse(t *testing.T) {
	resetViper()

	t.Setenv("TEST_BOOL_FALSE", "false")

	type Config struct {
		Disabled bool `env:"TEST_BOOL_FALSE" default:"true"`
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.Disabled {
		t.Errorf("Disabled = %v, want false", cfg.Disabled)
	}
}

func TestLoad_DeeplyNestedStructs(t *testing.T) {
	resetViper()

	t.Setenv("DEEP_VALUE", "deeply_nested")

	type Level3 struct {
		Value string `env:"DEEP_VALUE"`
	}

	type Level2 struct {
		Level3 Level3
	}

	type Level1 struct {
		Level2 Level2
	}

	type Config struct {
		Level1 Level1
	}

	var cfg Config
	err := LoadInto(&cfg)

	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	if cfg.Level1.Level2.Level3.Value != "deeply_nested" {
		t.Errorf("Level1.Level2.Level3.Value = %v, want %v", cfg.Level1.Level2.Level3.Value, "deeply_nested")
	}
}

func TestLoad_Generic(t *testing.T) {
	resetViper()

	t.Setenv("GENERIC_NAME", "generic_test")
	t.Setenv("GENERIC_PORT", "3000")

	type Config struct {
		Name string `env:"GENERIC_NAME" default:"default"`
		Port int    `env:"GENERIC_PORT" default:"8080"`
	}

	// Use the generic Load function
	cfg := Load[Config]()

	if cfg.Name != "generic_test" {
		t.Errorf("Name = %v, want %v", cfg.Name, "generic_test")
	}

	if cfg.Port != 3000 {
		t.Errorf("Port = %v, want %v", cfg.Port, 3000)
	}
}

func TestLoad_GenericWithDefaults(t *testing.T) {
	resetViper()

	type Config struct {
		Name string `env:"GENERIC_DEFAULT_NAME" default:"default_name"`
		Port int    `env:"GENERIC_DEFAULT_PORT" default:"8080"`
	}

	// Use the generic Load function with defaults
	cfg := Load[Config]()

	if cfg.Name != "default_name" {
		t.Errorf("Name = %v, want %v", cfg.Name, "default_name")
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %v, want %v", cfg.Port, 8080)
	}
}
