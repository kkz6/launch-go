package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000009_create_scripts_table",
		Name:      "Create scripts table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 9, 0, time.UTC),
		Up:        createScriptsTableUp,
		Down:      createScriptsTableDown,
	})
}

// scriptMigration model for migration
type scriptMigration struct {
	ID        string     `gorm:"type:char(26);primaryKey"`
	UserID    string     `gorm:"column:user_id;type:char(26);not null;index"`
	Name      string     `gorm:"type:varchar(255);not null"`
	Content   string     `gorm:"type:longtext;not null"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (scriptMigration) TableName() string {
	return "scripts"
}

// scriptWithUserFK defines the user foreign key
type scriptWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (scriptWithUserFK) TableName() string {
	return "scripts"
}

func createScriptsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&scriptMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&scriptWithUserFK{}, "User"); err != nil {
		return err
	}

	return nil
}

func createScriptsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&scriptMigration{})
}
