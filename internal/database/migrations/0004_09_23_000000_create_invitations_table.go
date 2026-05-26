package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0004_09_23_000000_create_invitations_table",
		Name:      "Create invitations table",
		Timestamp: time.Date(2004, 9, 23, 0, 0, 0, 0, time.UTC),
		Up:        createInvitationsTableUp,
		Down:      createInvitationsTableDown,
	})
}

// invitationMigration model for migration
type invitationMigration struct {
	ID              uint       `gorm:"primaryKey;autoIncrement"`
	Email           string     `gorm:"type:varchar(255);not null;uniqueIndex"`
	InvitationToken *string    `gorm:"column:invitation_token;type:varchar(32);uniqueIndex"`
	RegisteredAt    *time.Time `gorm:"column:registered_at;type:timestamp null"`
	TrialPeriod     *int       `gorm:"column:trial_period"`
	Comment         *string    `gorm:"type:text"`
	CreatedAt       *time.Time `gorm:"type:timestamp null"`
	UpdatedAt       *time.Time `gorm:"type:timestamp null"`
}

func (invitationMigration) TableName() string {
	return "invitations"
}

func createInvitationsTableUp(db *gorm.DB) error {
	return db.Migrator().CreateTable(&invitationMigration{})
}

func createInvitationsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&invitationMigration{})
}
