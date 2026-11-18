package migrations

import (
	"gorm.io/gorm"
	"time"
)

func init() {
	Register(Migration{ID: "0073_11_17_000000_git_deployment_authorization", Name: "Bind site deployments to repository authorization", Timestamp: time.Date(2026, 11, 17, 0, 0, 3, 0, time.UTC), Up: gitDeploymentAuthorizationUp})
}

func gitDeploymentAuthorizationUp(db *gorm.DB) error {
	for _, item := range []struct{ table, column, definition string }{
		{"deployments", "git_access", "JSONB NULL"},
		{"deployments", "webhook_key", "VARCHAR(128) NULL"},
		{"sites", "repository_dependency_ids", "JSONB NULL"},
	} {
		if !db.Migrator().HasColumn(item.table, item.column) {
			if err := db.Exec("ALTER TABLE " + item.table + " ADD COLUMN " + item.column + " " + item.definition).Error; err != nil {
				return err
			}
		}
	}
	return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_deployments_webhook_key ON deployments (webhook_key)").Error
}
