package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000000_create_tasks_table",
		Name:      "Create tasks table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 0, 0, time.UTC),
		Up:        createTasksTableUp,
		Down:      createTasksTableDown,
	})
}

// taskMigration model for migration (matches Laravel schema)
type taskMigration struct {
	ID        string     `gorm:"type:char(26);primaryKey"`
	ServerID  string     `gorm:"column:server_id;type:char(26);not null;index"`
	Name      string     `gorm:"type:varchar(255);not null"`
	User      string     `gorm:"type:varchar(255);not null"`
	Type      string     `gorm:"type:varchar(255);not null"`
	Instance  *string    `gorm:"type:text"`
	Script    string     `gorm:"type:text;not null"`
	Timeout   int        `gorm:"type:int;not null"`
	Status    string     `gorm:"type:varchar(255);not null"`
	Output    *string    `gorm:"type:text"`
	ExitCode  *int       `gorm:"type:int"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (taskMigration) TableName() string {
	return "tasks"
}

func createTasksTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&taskMigration{}); err != nil {
		return err
	}

	// Note: FK constraint to servers will be added after servers table is created
	return nil
}

func createTasksTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&taskMigration{})
}
