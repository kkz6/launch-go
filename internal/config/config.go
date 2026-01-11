package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Cors     CorsConfig
	Queue    QueueConfig
	SSH      SSHConfig
	Storage  StorageConfig
}

type AppConfig struct {
	Name        string
	Environment string
	Port        string
	Debug       bool
	URL         string
	Key         string // Encryption key (base64 encoded, same as Laravel APP_KEY)
}

type DatabaseConfig struct {
	Driver   string
	Host     string
	Port     string
	Database string
	Username string
	Password string
	SSLMode  string
}

func (d DatabaseConfig) DSN() string {
	switch d.Driver {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			d.Username, d.Password, d.Host, d.Port, d.Database)
	case "postgres":
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			d.Host, d.Port, d.Username, d.Password, d.Database, d.SSLMode)
	default:
		return ""
	}
}

type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret     string
	Expiration int // hours
}

type CorsConfig struct {
	AllowedOrigins string
}

type QueueConfig struct {
	Concurrency int
}

type SSHConfig struct {
	KeyPath    string
	DefaultUser string
}

type StorageConfig struct {
	Driver    string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Endpoint  string
}

func Load() (*Config, error) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("..")

	// Read .env file (optional)
	_ = viper.ReadInConfig()

	// Enable environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setDefaults()

	config := &Config{
		App: AppConfig{
			Name:        viper.GetString("APP_NAME"),
			Environment: viper.GetString("APP_ENV"),
			Port:        viper.GetString("APP_PORT"),
			Debug:       viper.GetBool("APP_DEBUG"),
			URL:         viper.GetString("APP_URL"),
			Key:         viper.GetString("APP_KEY"),
		},
		Database: DatabaseConfig{
			Driver:   viper.GetString("DB_DRIVER"),
			Host:     viper.GetString("DB_HOST"),
			Port:     viper.GetString("DB_PORT"),
			Database: viper.GetString("DB_DATABASE"),
			Username: viper.GetString("DB_USERNAME"),
			Password: viper.GetString("DB_PASSWORD"),
			SSLMode:  viper.GetString("DB_SSLMODE"),
		},
		Redis: RedisConfig{
			Address:  viper.GetString("REDIS_ADDRESS"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		JWT: JWTConfig{
			Secret:     viper.GetString("JWT_SECRET"),
			Expiration: viper.GetInt("JWT_EXPIRATION"),
		},
		Cors: CorsConfig{
			AllowedOrigins: viper.GetString("CORS_ALLOWED_ORIGINS"),
		},
		Queue: QueueConfig{
			Concurrency: viper.GetInt("QUEUE_CONCURRENCY"),
		},
		SSH: SSHConfig{
			KeyPath:     viper.GetString("SSH_KEY_PATH"),
			DefaultUser: viper.GetString("SSH_DEFAULT_USER"),
		},
		Storage: StorageConfig{
			Driver:    viper.GetString("STORAGE_DRIVER"),
			Bucket:    viper.GetString("STORAGE_BUCKET"),
			Region:    viper.GetString("STORAGE_REGION"),
			AccessKey: viper.GetString("STORAGE_ACCESS_KEY"),
			SecretKey: viper.GetString("STORAGE_SECRET_KEY"),
			Endpoint:  viper.GetString("STORAGE_ENDPOINT"),
		},
	}

	return config, nil
}

func setDefaults() {
	viper.SetDefault("APP_NAME", "Launch")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("APP_DEBUG", true)
	viper.SetDefault("APP_URL", "http://localhost:8080")

	viper.SetDefault("DB_DRIVER", "mysql")
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "3306")
	viper.SetDefault("DB_DATABASE", "launch")
	viper.SetDefault("DB_USERNAME", "root")
	viper.SetDefault("DB_PASSWORD", "")
	viper.SetDefault("DB_SSLMODE", "disable")

	viper.SetDefault("REDIS_ADDRESS", "localhost:6379")
	viper.SetDefault("REDIS_PASSWORD", "")
	viper.SetDefault("REDIS_DB", 0)

	viper.SetDefault("JWT_SECRET", "change-me-in-production")
	viper.SetDefault("JWT_EXPIRATION", 72)

	viper.SetDefault("CORS_ALLOWED_ORIGINS", "*")

	viper.SetDefault("QUEUE_CONCURRENCY", 10)

	viper.SetDefault("SSH_KEY_PATH", "~/.ssh")
	viper.SetDefault("SSH_DEFAULT_USER", "root")

	viper.SetDefault("STORAGE_DRIVER", "s3")
}
