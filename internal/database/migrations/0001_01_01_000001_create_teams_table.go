package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0001_01_01_000001_create_teams_table",
		Name:      "Create teams table",
		Timestamp: time.Date(2001, 1, 1, 0, 0, 1, 0, time.UTC),
		Up:        createTeamsTableUp,
		Down:      createTeamsTableDown,
	})
}

// teamMigration model for migration (matches Laravel schema exactly)
type teamMigration struct {
	ID                   string     `gorm:"type:char(26);primaryKey"`
	UserID               string     `gorm:"column:user_id;type:char(26);not null;index"`
	Name                 string     `gorm:"type:varchar(255);not null"`
	ImagePath            *string    `gorm:"column:image_path;type:varchar(255)"`
	PersonalTeam         bool       `gorm:"type:boolean;not null"`
	RequiresSubscription bool       `gorm:"type:boolean;not null;default:1"`
	CreatedAt            *time.Time `gorm:"type:timestamp null"`
	UpdatedAt            *time.Time `gorm:"type:timestamp null"`
}

func (teamMigration) TableName() string {
	return "teams"
}

// teamWithUserFK defines the foreign key relationship for teams.user_id -> users.id
type teamWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (teamWithUserFK) TableName() string {
	return "teams"
}

// userWithTeamFK defines the foreign key relationship for users.current_team_id -> teams.id
type userWithTeamFK struct {
	CurrentTeamID *string        `gorm:"column:current_team_id"`
	CurrentTeam   *teamMigration `gorm:"foreignKey:CurrentTeamID;references:ID;constraint:OnDelete:SET NULL"`
}

func (userWithTeamFK) TableName() string {
	return "users"
}

func createTeamsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	// Create teams table
	if err := migrator.CreateTable(&teamMigration{}); err != nil {
		return err
	}

	// Add foreign key constraint for teams.user_id -> users.id
	if err := migrator.CreateConstraint(&teamWithUserFK{}, "User"); err != nil {
		return err
	}

	// Add foreign key constraint for users.current_team_id -> teams.id
	return migrator.CreateConstraint(&userWithTeamFK{}, "CurrentTeam")
}

func createTeamsTableDown(db *gorm.DB) error {
	migrator := db.Migrator()

	// Drop foreign key from users table
	_ = migrator.DropConstraint(&userWithTeamFK{}, "CurrentTeam")

	// Drop teams table (will cascade drop teams_user_id_foreign)
	return migrator.DropTable(&teamMigration{})
}
