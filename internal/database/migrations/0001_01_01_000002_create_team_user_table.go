package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0001_01_01_000002_create_team_user_table",
		Name:      "Create team_user pivot table",
		Timestamp: time.Date(2001, 1, 1, 0, 0, 2, 0, time.UTC),
		Up:        createTeamUserTableUp,
		Down:      createTeamUserTableDown,
	})
}

// teamUserMigration model for migration (pivot table, matches Laravel schema exactly)
type teamUserMigration struct {
	ID        uint64     `gorm:"type:bigint unsigned;primaryKey;autoIncrement"`
	TeamID    string     `gorm:"column:team_id;type:char(26);not null;uniqueIndex:team_user_team_id_user_id_unique,priority:1"`
	UserID    string     `gorm:"column:user_id;type:char(26);not null;uniqueIndex:team_user_team_id_user_id_unique,priority:2"`
	Role      *string    `gorm:"type:varchar(255)"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (teamUserMigration) TableName() string {
	return "team_user"
}

// teamUserWithFK defines the foreign key relationships for team_user table
type teamUserWithFK struct {
	TeamID string          `gorm:"column:team_id"`
	UserID string          `gorm:"column:user_id"`
	Team   *teamMigration  `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (teamUserWithFK) TableName() string {
	return "team_user"
}

func createTeamUserTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	// Create team_user table
	if err := migrator.CreateTable(&teamUserMigration{}); err != nil {
		return err
	}

	// Add foreign key constraint for team_user.team_id -> teams.id
	if err := migrator.CreateConstraint(&teamUserWithFK{}, "Team"); err != nil {
		return err
	}

	// Add foreign key constraint for team_user.user_id -> users.id
	if err := migrator.CreateConstraint(&teamUserWithFK{}, "User"); err != nil {
		return err
	}

	return nil
}

func createTeamUserTableDown(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.DropTable(&teamUserMigration{}); err != nil {
		return err
	}

	return nil
}
