package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0009_01_01_000000_create_activity_log_table",
		Name:      "Create activity_log table",
		Timestamp: time.Date(2009, 1, 1, 0, 0, 0, 0, time.UTC),
		Up:        createActivityLogTableUp,
		Down:      createActivityLogTableDown,
	})
}

type activityLogMigration struct {
	ID          string     `gorm:"type:char(26);primaryKey"`
	LogName     string     `gorm:"type:varchar(255);index;not null;default:'default'"`
	Description string     `gorm:"type:text;not null"`
	SubjectType *string    `gorm:"type:varchar(255);index"`
	SubjectID   *string    `gorm:"type:char(26);index"`
	CauserType  *string    `gorm:"type:varchar(255);index"`
	CauserID    *string    `gorm:"type:char(26);index"`
	Properties  *string    `gorm:"type:json"`
	Event       *string    `gorm:"type:varchar(255)"`
	BatchUUID   *string    `gorm:"type:char(36);index"`
	CreatedAt   *time.Time `gorm:"type:timestamp null"`
	UpdatedAt   *time.Time `gorm:"type:timestamp null"`
}

func (activityLogMigration) TableName() string {
	return "activity_log"
}

func createActivityLogTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	return migrator.CreateTable(&activityLogMigration{})
}

func createActivityLogTableDown(db *gorm.DB) error {
	migrator := db.Migrator()

	return migrator.DropTable(&activityLogMigration{})
}
