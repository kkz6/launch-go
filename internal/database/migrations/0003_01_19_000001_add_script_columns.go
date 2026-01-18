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
		Down:      addScriptColumnsDown,
	})
}

func addScriptColumnsUp(db *gorm.DB) error {
	// Add team_id column (nullable for personal scripts)
	if err := db.Exec("ALTER TABLE scripts ADD COLUMN team_id CHAR(26) NULL AFTER user_id").Error; err != nil {
		return err
	}

	// Add user column (unix user to run as)
	if err := db.Exec("ALTER TABLE scripts ADD COLUMN user VARCHAR(255) NOT NULL DEFAULT 'root' AFTER name").Error; err != nil {
		return err
	}

	// Add index for team_id
	if err := db.Exec("CREATE INDEX idx_scripts_team_id ON scripts(team_id)").Error; err != nil {
		return err
	}

	// Add foreign key for team_id
	if err := db.Exec("ALTER TABLE scripts ADD CONSTRAINT fk_scripts_team FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE").Error; err != nil {
		return err
	}

	return nil
}

func addScriptColumnsDown(db *gorm.DB) error {
	db.Exec("ALTER TABLE scripts DROP FOREIGN KEY fk_scripts_team")
	db.Exec("DROP INDEX idx_scripts_team_id ON scripts")
	db.Exec("ALTER TABLE scripts DROP COLUMN user")
	db.Exec("ALTER TABLE scripts DROP COLUMN team_id")
	return nil
}
