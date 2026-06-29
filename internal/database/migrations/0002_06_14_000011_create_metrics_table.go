package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000011_create_metrics_table",
		Name:      "Create metrics table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 11, 0, time.UTC),
		Up:        createMetricsTableUp,
	})
}

// metricMigration model for migration
type metricMigration struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	ServerID    string     `gorm:"column:server_id;type:char(26);not null;index"`
	Load        float64    `gorm:"type:decimal(5,2);not null"`
	MemoryTotal float64    `gorm:"column:memory_total;type:decimal(15,0);not null"`
	MemoryUsed  float64    `gorm:"column:memory_used;type:decimal(15,0);not null"`
	MemoryFree  float64    `gorm:"column:memory_free;type:decimal(15,0);not null"`
	DiskTotal   float64    `gorm:"column:disk_total;type:decimal(15,0);not null"`
	DiskUsed    float64    `gorm:"column:disk_used;type:decimal(15,0);not null"`
	DiskFree    float64    `gorm:"column:disk_free;type:decimal(15,0);not null"`
	CreatedAt   *time.Time `gorm:"type:timestamp null"`
	UpdatedAt   *time.Time `gorm:"type:timestamp null"`
}

func (metricMigration) TableName() string {
	return "metrics"
}

// metricWithServerFK defines the server foreign key
type metricWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (metricWithServerFK) TableName() string {
	return "metrics"
}

func createMetricsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&metricMigration{}); err != nil {
		return err
	}

	return migrator.CreateConstraint(&metricWithServerFK{}, "Server")
}
