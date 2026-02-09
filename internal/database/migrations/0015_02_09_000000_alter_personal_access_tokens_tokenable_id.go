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
	return db.Exec("ALTER TABLE `personal_access_tokens` MODIFY `tokenable_id` varchar(26) NOT NULL").Error
}
