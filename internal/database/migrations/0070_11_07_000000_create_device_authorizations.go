package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0070_11_07_000000_create_device_authorizations",
		Name:      "Create device authorization grants",
		Timestamp: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC),
		Up:        createDeviceAuthorizationsUp,
	})
}

func createDeviceAuthorizationsUp(db *gorm.DB) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS device_authorizations (
			id VARCHAR(26) PRIMARY KEY,
			created_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL,
			device_code_hash VARCHAR(64) NOT NULL UNIQUE,
			user_code_hash VARCHAR(64) NOT NULL UNIQUE,
			user_id VARCHAR(26) NULL,
			status VARCHAR(20) NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			last_polled_at TIMESTAMP NULL,
			authorization_attempts INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS device_authorization_limits (
			id VARCHAR(64) PRIMARY KEY,
			attempts INTEGER NOT NULL,
			expires_at TIMESTAMP NOT NULL
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	for _, table := range []string{"device_authorizations", "device_authorization_limits"} {
		index := "idx_" + table + "_expires_at"
		if !db.Migrator().HasIndex(table, index) {
			if err := db.Exec("CREATE INDEX " + index + " ON " + table + " (expires_at)").Error; err != nil {
				return err
			}
		}
	}
	return nil
}
