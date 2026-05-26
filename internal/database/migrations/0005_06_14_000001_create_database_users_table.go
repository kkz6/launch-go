package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0005_06_14_000001_create_database_users_table",
		Name:      "Create database_users table",
		Timestamp: time.Date(2005, 6, 14, 0, 0, 1, 0, time.UTC),
		Up:        createDatabaseUsersTableUp,
		Down:      createDatabaseUsersTableDown,
	})
}

// databaseUserMigration model for migration
type databaseUserMigration struct {
	ID                        string     `gorm:"type:char(26);primaryKey"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index"`
	Name                      string     `gorm:"type:varchar(255);not null"`
	Password                  *string    `gorm:"type:text"` // encrypted
	Host                      string     `gorm:"type:varchar(255);not null;default:localhost"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null"`
}

func (databaseUserMigration) TableName() string {
	return "database_users"
}

// databaseUserWithServerFK defines the server foreign key
type databaseUserWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (databaseUserWithServerFK) TableName() string {
	return "database_users"
}

func createDatabaseUsersTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&databaseUserMigration{}); err != nil {
		return err
	}

	return migrator.CreateConstraint(&databaseUserWithServerFK{}, "Server")
}

func createDatabaseUsersTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&databaseUserMigration{})
}
