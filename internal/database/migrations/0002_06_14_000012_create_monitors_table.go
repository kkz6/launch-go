package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000012_create_monitors_table",
		Name:      "Create monitors table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 12, 0, time.UTC),
		Up:        createMonitorsTableUp,
	})
}

// monitorMigration model for migration
type monitorMigration struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement"`
	ServerID  string     `gorm:"column:server_id;type:char(26);not null;index"`
	Type      string     `gorm:"type:varchar(255);not null"`
	Operator  string     `gorm:"type:varchar(255);not null"`
	Threshold float64    `gorm:"type:decimal(8,2);not null"`
	Duration  int        `gorm:"type:int;not null"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (monitorMigration) TableName() string {
	return "monitors"
}

// monitorWithServerFK defines the server foreign key
type monitorWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (monitorWithServerFK) TableName() string {
	return "monitors"
}

func createMonitorsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&monitorMigration{}); err != nil {
		return err
	}

	return migrator.CreateConstraint(&monitorWithServerFK{}, "Server")
}
