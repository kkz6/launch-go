package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0005_06_14_000000_create_databases_table",
		Name:      "Create databases table",
		Timestamp: time.Date(2005, 6, 14, 0, 0, 0, 0, time.UTC),
		Up:        createDatabasesTableUp,
		Down:      createDatabasesTableDown,
	})
}

// databaseMigration model for migration (note: named to avoid conflict with gorm.DB)
type databaseMigration struct {
	ID                        string     `gorm:"type:char(26);primaryKey"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index"`
	Name                      string     `gorm:"type:varchar(255);not null"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null"`
}

func (databaseMigration) TableName() string {
	return "databases"
}

// databaseWithServerFK defines the server foreign key
type databaseWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (databaseWithServerFK) TableName() string {
	return "databases"
}

func createDatabasesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&databaseMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&databaseWithServerFK{}, "Server"); err != nil {
		return err
	}

	return nil
}

func createDatabasesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&databaseMigration{})
}
