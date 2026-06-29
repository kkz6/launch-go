package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_06_14_000005_create_commands_table",
		Name:      "Create commands table",
		Timestamp: time.Date(2003, 6, 14, 0, 0, 5, 0, time.UTC),
		Up:        createCommandsTableUp,
	})
}

// commandMigration model for migration
type commandMigration struct {
	ID        string     `gorm:"type:char(26);primaryKey"`
	SiteID    string     `gorm:"column:site_id;type:char(26);not null;index"`
	UserID    string     `gorm:"column:user_id;type:char(26);not null;index"`
	Command   string     `gorm:"type:varchar(255);not null"`
	Status    string     `gorm:"type:varchar(255);not null"`
	Output    *string    `gorm:"type:text"`
	ExitCode  *int       `gorm:"column:exit_code"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (commandMigration) TableName() string {
	return "commands"
}

// commandWithSiteFK defines the site foreign key
type commandWithSiteFK struct {
	SiteID string         `gorm:"column:site_id"`
	Site   *siteMigration `gorm:"foreignKey:SiteID;references:ID;constraint:OnDelete:CASCADE"`
}

func (commandWithSiteFK) TableName() string {
	return "commands"
}

// commandWithUserFK defines the user foreign key
type commandWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (commandWithUserFK) TableName() string {
	return "commands"
}

func createCommandsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&commandMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&commandWithSiteFK{}, "Site"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&commandWithUserFK{}, "User")
}
