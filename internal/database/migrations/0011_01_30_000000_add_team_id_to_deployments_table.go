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
	// Check if column already exists (for fresh installs where 0010 already added it)
	var count int64
	db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'deployments' AND column_name = 'team_id'").Scan(&count)
	if count > 0 {
		return nil
	}

	// Add nullable column
	if err := db.Exec("ALTER TABLE `deployments` ADD COLUMN team_id CHAR(26) NULL AFTER site_id").Error; err != nil {
		return fmt.Errorf("failed to add team_id column: %w", err)
	}

	// Backfill from sites table
	if err := db.Exec("UPDATE `deployments` t INNER JOIN `sites` p ON t.site_id = p.id SET t.team_id = p.team_id").Error; err != nil {
		return fmt.Errorf("failed to backfill team_id: %w", err)
	}

	// Make NOT NULL
	if err := db.Exec("ALTER TABLE `deployments` MODIFY COLUMN team_id CHAR(26) NOT NULL").Error; err != nil {
		return fmt.Errorf("failed to make team_id NOT NULL: %w", err)
	}

	// Add index
	if err := db.Exec("CREATE INDEX idx_deployments_team_id ON `deployments`(team_id)").Error; err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	// Add foreign key
	return db.Exec("ALTER TABLE `deployments` ADD CONSTRAINT fk_deployments_team_id FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE").Error
}

func addTeamIDToDeploymentsDown(db *gorm.DB) error {
	db.Exec("ALTER TABLE `deployments` DROP FOREIGN KEY fk_deployments_team_id")
	db.Exec("DROP INDEX idx_deployments_team_id ON `deployments`")
	db.Exec("ALTER TABLE `deployments` DROP COLUMN team_id")
	return nil
}
