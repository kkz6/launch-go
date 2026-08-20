package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0067_08_20_000000_add_locale_to_users",
		Name:      "Add locale to users",
		Timestamp: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		Up:        addLocaleToUsersUp,
	})
}

// A NULL locale means automatic language detection from Accept-Language.
func addLocaleToUsersUp(db *gorm.DB) error {
	return db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS locale VARCHAR(10) NULL").Error
}
