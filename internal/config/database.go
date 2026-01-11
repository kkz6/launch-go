package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver     string
	Host       string
	Port       string
	Database   string
	Username   string
	Password   string
	SSLMode    string
	LogQueries bool
}

// DSN returns the database connection string
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

func loadDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Driver:     viper.GetString("DB_DRIVER"),
		Host:       viper.GetString("DB_HOST"),
		Port:       viper.GetString("DB_PORT"),
		Database:   viper.GetString("DB_DATABASE"),
		Username:   viper.GetString("DB_USERNAME"),
		Password:   viper.GetString("DB_PASSWORD"),
		SSLMode:    viper.GetString("DB_SSLMODE"),
		LogQueries: viper.GetBool("DB_LOG_QUERIES"),
	}
}

func setDatabaseDefaults() {
	viper.SetDefault("DB_DRIVER", "mysql")
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "3306")
	viper.SetDefault("DB_DATABASE", "launch")
	viper.SetDefault("DB_USERNAME", "root")
	viper.SetDefault("DB_PASSWORD", "")
	viper.SetDefault("DB_SSLMODE", "disable")
	viper.SetDefault("DB_LOG_QUERIES", false)
}
