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

// Team model for migration (matches Laravel schema exactly)
type teamMigration struct {
	ID                   string     `gorm:"type:char(26);primaryKey"`
	UserID               string     `gorm:"column:user_id;type:char(26);not null;index"`
	Name                 string     `gorm:"type:varchar(255);not null"`
	PersonalTeam         bool       `gorm:"type:tinyint(1);not null"`
	RequiresSubscription bool       `gorm:"type:tinyint(1);not null;default:1"`
	CreatedAt            *time.Time `gorm:"type:timestamp null"`
	UpdatedAt            *time.Time `gorm:"type:timestamp null"`
}

func (teamMigration) TableName() string {
	return "teams"
}

func createTeamsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	// Create teams table
	if err := migrator.CreateTable(&teamMigration{}); err != nil {
		return err
	}

	// Add foreign key constraint for user_id -> users.id
	if err := db.Exec(`
		ALTER TABLE teams ADD CONSTRAINT teams_user_id_foreign
		FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
	`).Error; err != nil {
		return err
	}

	// Add foreign key constraint for users.current_team_id -> teams.id
	if err := db.Exec(`
		ALTER TABLE users ADD CONSTRAINT users_current_team_id_foreign
		FOREIGN KEY (current_team_id) REFERENCES teams (id) ON DELETE SET NULL
	`).Error; err != nil {
		return err
	}

	return nil
}

func createTeamsTableDown(db *gorm.DB) error {
	migrator := db.Migrator()

	// Drop foreign key from users table
	_ = db.Exec("ALTER TABLE users DROP FOREIGN KEY users_current_team_id_foreign").Error

	// Drop teams table
	if err := migrator.DropTable(&teamMigration{}); err != nil {
		return err
	}

	return nil
}
