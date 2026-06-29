package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0058_06_03_000001_drop_spatie_tables",
		Name:      "Drop dormant Spatie permission tables (replaced by users.staff_role)",
		Timestamp: time.Date(2026, 6, 3, 0, 0, 1, 0, time.UTC),
		Up:        dropSpatieTablesUp,
	})
}

func dropSpatieTablesUp(db *gorm.DB) error {
	// FK-safe order: junction tables first, then roles/permissions. The Spatie
	// permission system is retired — its only live use (global admin/manager →
	// subscription bypass) now reads users.staff_role.
	stmts := []string{
		"DROP TABLE IF EXISTS role_has_permissions",
		"DROP TABLE IF EXISTS model_has_permissions",
		"DROP TABLE IF EXISTS model_has_roles",
		"DROP TABLE IF EXISTS roles",
		"DROP TABLE IF EXISTS permissions",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
