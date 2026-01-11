package database

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/pkg/logger"
)

// InitEncryption initializes the encryption key for the serializers
func InitEncryption(appKey string) error {
	if appKey == "" {
		return nil // Encryption not configured
	}
	return serializers.SetEncryptionKeyFromBase64(appKey)
}

// Connect establishes a database connection with custom logging
func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	return ConnectWithLogger(cfg, nil)
}

// ConnectWithLogger establishes a database connection with a custom zerolog logger
func ConnectWithLogger(cfg config.DatabaseConfig, appLogger *zerolog.Logger) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "mysql":
		dialector = mysql.Open(cfg.DSN())
	case "postgres":
		dialector = postgres.Open(cfg.DSN())
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	// Configure GORM
	gormConfig := &gorm.Config{
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	// Only enable query logging if explicitly configured
	if appLogger != nil && cfg.LogQueries {
		gormConfig.Logger = logger.NewGormLogger(appLogger)
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}
