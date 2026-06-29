package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0015_02_09_000000_add_source_control_unique_constraints",
		Name:      "Add unique constraints to source_controls and source_control_repositories",
		Timestamp: time.Date(2015, 2, 9, 0, 0, 0, 0, time.UTC),
		Up:        addSourceControlUniqueConstraintsUp,
	})
}

func addSourceControlUniqueConstraintsUp(db *gorm.DB) error {
	// Unique index: one installation per team per provider
	if err := db.Exec(
		"CREATE UNIQUE INDEX idx_source_controls_provider_providerid_teamid ON source_controls (provider, provider_id, team_id)",
	).Error; err != nil {
		return err
	}

	// Unique index: one repository per source control per full_name
	return db.Exec(
		"CREATE UNIQUE INDEX idx_source_control_repos_scid_fullname ON source_control_repositories (source_control_id, full_name)",
	).Error
}
