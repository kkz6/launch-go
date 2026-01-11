package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0001_01_13_000000_create_passkeys_table",
		Name:      "Create passkeys table",
		Timestamp: time.Date(2001, 1, 13, 0, 0, 0, 0, time.UTC),
		Up:        createPasskeysTableUp,
		Down:      createPasskeysTableDown,
	})
}

// passkeyMigration model for migration
type passkeyMigration struct {
	ID              string     `gorm:"type:char(26);primaryKey"`
	UserID          string     `gorm:"column:user_id;type:char(26);not null;index:idx_passkeys_user_created,priority:1"`
	Name            *string    `gorm:"type:varchar(255)"`
	CredentialID    string     `gorm:"column:credential_id;type:varchar(255);not null;uniqueIndex"`
	PublicKey       string     `gorm:"column:public_key;type:text;not null"`
	SignCount       int        `gorm:"column:sign_count;type:int;not null;default:0"`
	AAGUID          *string    `gorm:"type:varchar(255)"`
	Transports      *string    `gorm:"type:json"`
	Type            string     `gorm:"type:varchar(255);not null;default:public-key"`
	AttestationData *string    `gorm:"column:attestation_data;type:json"`
	LastUsedAt      *time.Time `gorm:"column:last_used_at;type:timestamp null"`
	CreatedAt       *time.Time `gorm:"type:timestamp null;index:idx_passkeys_user_created,priority:2"`
	UpdatedAt       *time.Time `gorm:"type:timestamp null"`
}

func (passkeyMigration) TableName() string {
	return "passkeys"
}

// passkeyWithUserFK defines the user foreign key
type passkeyWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (passkeyWithUserFK) TableName() string {
	return "passkeys"
}

func createPasskeysTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&passkeyMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&passkeyWithUserFK{}, "User"); err != nil {
		return err
	}

	return nil
}

func createPasskeysTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&passkeyMigration{})
}
