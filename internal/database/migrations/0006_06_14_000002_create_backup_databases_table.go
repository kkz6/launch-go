package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0006_06_14_000002_create_backup_databases_table",
		Name:      "Create backup_databases pivot table",
		Timestamp: time.Date(2006, 6, 14, 0, 0, 2, 0, time.UTC),
		Up:        createBackupDatabasesTableUp,
		Down:      createBackupDatabasesTableDown,
	})
}

// backupDatabaseMigration model for pivot table
type backupDatabaseMigration struct {
	BackupID   string `gorm:"column:backup_id;type:char(26);not null;uniqueIndex:idx_backup_database"`
	DatabaseID string `gorm:"column:database_id;type:char(26);not null;uniqueIndex:idx_backup_database"`
}

func (backupDatabaseMigration) TableName() string {
	return "backup_databases"
}

// backupDatabaseWithBackupFK defines the backup foreign key
type backupDatabaseWithBackupFK struct {
	BackupID string           `gorm:"column:backup_id"`
	Backup   *backupMigration `gorm:"foreignKey:BackupID;references:ID;constraint:OnDelete:CASCADE"`
}

func (backupDatabaseWithBackupFK) TableName() string {
	return "backup_databases"
}

// backupDatabaseWithDatabaseFK defines the database foreign key
type backupDatabaseWithDatabaseFK struct {
	DatabaseID string             `gorm:"column:database_id"`
	Database   *databaseMigration `gorm:"foreignKey:DatabaseID;references:ID;constraint:OnDelete:CASCADE"`
}

func (backupDatabaseWithDatabaseFK) TableName() string {
	return "backup_databases"
}

func createBackupDatabasesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&backupDatabaseMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&backupDatabaseWithBackupFK{}, "Backup"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&backupDatabaseWithDatabaseFK{}, "Database")
}

func createBackupDatabasesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&backupDatabaseMigration{})
}
