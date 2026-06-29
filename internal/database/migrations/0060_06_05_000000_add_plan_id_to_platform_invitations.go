package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0060_06_05_000000_add_plan_id_to_platform_invitations",
		Name:      "Add plan_id to platform_invitations",
		Timestamp: time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		Up:        addPlanIDToPlatformInvitationsUp,
	})
}

func addPlanIDToPlatformInvitationsUp(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE platform_invitations ADD COLUMN plan_id VARCHAR(50) NOT NULL DEFAULT ''",
	).Error
}
