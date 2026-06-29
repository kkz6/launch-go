package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0001_01_01_000004_create_personal_access_tokens_table",
		Name:      "Create personal_access_tokens table",
		Timestamp: time.Date(2001, 1, 1, 0, 0, 4, 0, time.UTC),
		Up:        createPersonalAccessTokensTableUp,
	})
}

// PersonalAccessToken model for migration (matches Laravel schema exactly)
type personalAccessTokenMigration struct {
	ID            string     `gorm:"type:char(26);primaryKey"`
	TokenableType string     `gorm:"column:tokenable_type;type:varchar(255);not null;index:personal_access_tokens_tokenable_type_tokenable_id_index,priority:1"`
	TokenableID   uint64     `gorm:"column:tokenable_id;type:bigint;not null;index:personal_access_tokens_tokenable_type_tokenable_id_index,priority:2"`
	Name          string     `gorm:"type:varchar(255);not null"`
	Token         string     `gorm:"type:varchar(64);uniqueIndex;not null"`
	Abilities     *string    `gorm:"type:text"`
	LastUsedAt    *time.Time `gorm:"type:timestamp null"`
	ExpiresAt     *time.Time `gorm:"type:timestamp null"`
	CreatedAt     *time.Time `gorm:"type:timestamp null"`
	UpdatedAt     *time.Time `gorm:"type:timestamp null"`
}

func (personalAccessTokenMigration) TableName() string {
	return "personal_access_tokens"
}

func createPersonalAccessTokensTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	// Create personal_access_tokens table
	return migrator.CreateTable(&personalAccessTokenMigration{})
}
