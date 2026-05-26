package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0015_02_09_000000_alter_personal_access_tokens_tokenable_id",
		Name:      "Change tokenable_id from bigint to varchar for ULID support",
		Timestamp: time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC),
		Up:        alterPersonalAccessTokensTokenableIDUp,
	})
}

func alterPersonalAccessTokensTokenableIDUp(db *gorm.DB) error {
	// Postgres splits the column-type change and the NOT NULL constraint.
	// USING tokenable_id::varchar handles the bigint → varchar coercion
	// for any rows that existed before the change.
	if err := db.Exec(`ALTER TABLE personal_access_tokens ALTER COLUMN tokenable_id TYPE varchar(26) USING tokenable_id::varchar`).Error; err != nil {
		return err
	}
	return db.Exec(`ALTER TABLE personal_access_tokens ALTER COLUMN tokenable_id SET NOT NULL`).Error
}
