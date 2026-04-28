package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0025_04_28_000000_add_polish_to_docker_apps",
		Name:      "Add health/limits/auto-redeploy to docker_apps",
		Timestamp: time.Date(2026, 4, 28, 0, 0, 2, 0, time.UTC),
		Up:        addPolishToDockerAppsUp,
		Down:      addPolishToDockerAppsDown,
	})
}

func addPolishToDockerAppsUp(db *gorm.DB) error {
	stmts := []string{
		"ALTER TABLE docker_apps ADD COLUMN health_cmd VARCHAR(255) NULL",
		"ALTER TABLE docker_apps ADD COLUMN health_interval_seconds INT NULL",
		"ALTER TABLE docker_apps ADD COLUMN health_timeout_seconds INT NULL",
		"ALTER TABLE docker_apps ADD COLUMN health_retries INT NULL",
		"ALTER TABLE docker_apps ADD COLUMN memory_limit VARCHAR(32) NULL",
		"ALTER TABLE docker_apps ADD COLUMN cpu_limit VARCHAR(32) NULL",
		"ALTER TABLE docker_apps ADD COLUMN auto_redeploy_cron VARCHAR(64) NULL",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}

func addPolishToDockerAppsDown(db *gorm.DB) error {
	stmts := []string{
		"ALTER TABLE docker_apps DROP COLUMN auto_redeploy_cron",
		"ALTER TABLE docker_apps DROP COLUMN cpu_limit",
		"ALTER TABLE docker_apps DROP COLUMN memory_limit",
		"ALTER TABLE docker_apps DROP COLUMN health_retries",
		"ALTER TABLE docker_apps DROP COLUMN health_timeout_seconds",
		"ALTER TABLE docker_apps DROP COLUMN health_interval_seconds",
		"ALTER TABLE docker_apps DROP COLUMN health_cmd",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
