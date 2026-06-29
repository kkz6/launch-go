package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0061_06_29_000000_add_token_expires_at_to_source_controls",
		Name:      "Add token_expires_at to source_controls",
		Timestamp: time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC),
		Up:        addTokenExpiresAtToSourceControlsUp,
	})
}

// The source_controls model carries token_expires_at, but databases that
// ran the original create_source_controls_table migration (before that
// column was added to its definition) never got the column — so inserting
// a GitHub App installation failed with "column token_expires_at does not
// exist". IF NOT EXISTS keeps this a no-op on fresh databases.
func addTokenExpiresAtToSourceControlsUp(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE source_controls ADD COLUMN IF NOT EXISTS token_expires_at TIMESTAMPTZ",
	).Error
}
