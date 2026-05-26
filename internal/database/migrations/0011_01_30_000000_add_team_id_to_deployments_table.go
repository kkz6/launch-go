package migrations

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0011_01_30_000000_add_team_id_to_deployments_table",
		Name:      "Add team_id to deployments table",
		Timestamp: time.Date(2011, 1, 30, 0, 0, 0, 0, time.UTC),
		Up:        addTeamIDToDeploymentsUp,
		Down:      addTeamIDToDeploymentsDown,
	})
}

func addTeamIDToDeploymentsUp(db *gorm.DB) error {
	// Check if column already exists (for fresh installs where 0010 already added it).
	// Postgres uses current_schema() instead of MySQL's current_schema() function.
	var count int64
	db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'deployments' AND column_name = 'team_id'`).Scan(&count)
	if count > 0 {
		return nil
	}

	// Add nullable column
	if err := db.Exec(`ALTER TABLE deployments ADD COLUMN team_id CHAR(26) NULL`).Error; err != nil {
		return fmt.Errorf("failed to add team_id column: %w", err)
	}

	// Backfill from sites table — Postgres UPDATE...FROM (no INNER JOIN in UPDATE)
	if err := db.Exec(`UPDATE deployments t SET team_id = p.team_id FROM sites p WHERE t.site_id = p.id`).Error; err != nil {
		return fmt.Errorf("failed to backfill team_id: %w", err)
	}

	// Make NOT NULL — Postgres splits the column attribute from the type.
	if err := db.Exec(`ALTER TABLE deployments ALTER COLUMN team_id SET NOT NULL`).Error; err != nil {
		return fmt.Errorf("failed to make team_id NOT NULL: %w", err)
	}

	// Add index
	if err := db.Exec(`CREATE INDEX idx_deployments_team_id ON deployments(team_id)`).Error; err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return db.Exec(`ALTER TABLE deployments ADD CONSTRAINT fk_deployments_team_id FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE`).Error
}

func addTeamIDToDeploymentsDown(db *gorm.DB) error {
	db.Exec(`ALTER TABLE deployments DROP CONSTRAINT IF EXISTS fk_deployments_team_id`)
	db.Exec(`DROP INDEX IF EXISTS idx_deployments_team_id`)
	db.Exec(`ALTER TABLE deployments DROP COLUMN IF EXISTS team_id`)
	return nil
}
