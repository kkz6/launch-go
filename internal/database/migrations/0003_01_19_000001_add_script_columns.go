package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_01_19_000001_add_script_columns",
		Name:      "Add columns to scripts table",
		Timestamp: time.Date(2003, 1, 19, 0, 0, 1, 0, time.UTC),
		Up:        addScriptColumnsUp,
	})
}

func addScriptColumnsUp(db *gorm.DB) error {
	// Postgres doesn't support `AFTER column_name`; columns land at the end.
	// Column order doesn't affect semantics so the are dropped.
	if err := db.Exec(`ALTER TABLE scripts ADD COLUMN team_id CHAR(26) NULL`).Error; err != nil {
		return err
	}

	if err := db.Exec(`ALTER TABLE scripts ADD COLUMN "user" VARCHAR(255) NOT NULL DEFAULT 'root'`).Error; err != nil {
		return err
	}

	if err := db.Exec(`CREATE INDEX idx_scripts_team_id ON scripts(team_id)`).Error; err != nil {
		return err
	}

	return db.Exec(`ALTER TABLE scripts ADD CONSTRAINT fk_scripts_team FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE`).Error
}
