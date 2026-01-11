package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0001_01_01_000003_create_team_invitations_table",
		Name:      "Create team_invitations table",
		Timestamp: time.Date(2001, 1, 1, 0, 0, 3, 0, time.UTC),
		Up:        createTeamInvitationsTableUp,
		Down:      createTeamInvitationsTableDown,
	})
}

// teamInvitationMigration model for migration (matches Laravel schema exactly)
type teamInvitationMigration struct {
	ID        string     `gorm:"type:char(26);primaryKey"`
	TeamID    string     `gorm:"column:team_id;type:char(26);not null;uniqueIndex:team_invitations_team_id_email_unique,priority:1"`
	Email     string     `gorm:"type:varchar(255);not null;uniqueIndex:team_invitations_team_id_email_unique,priority:2"`
	Role      *string    `gorm:"type:varchar(255)"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (teamInvitationMigration) TableName() string {
	return "team_invitations"
}

// teamInvitationWithFK defines the foreign key relationship for team_invitations.team_id -> teams.id
type teamInvitationWithFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (teamInvitationWithFK) TableName() string {
	return "team_invitations"
}

func createTeamInvitationsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	// Create team_invitations table
	if err := migrator.CreateTable(&teamInvitationMigration{}); err != nil {
		return err
	}

	// Add foreign key constraint for team_invitations.team_id -> teams.id
	if err := migrator.CreateConstraint(&teamInvitationWithFK{}, "Team"); err != nil {
		return err
	}

	return nil
}

func createTeamInvitationsTableDown(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.DropTable(&teamInvitationMigration{}); err != nil {
		return err
	}

	return nil
}
