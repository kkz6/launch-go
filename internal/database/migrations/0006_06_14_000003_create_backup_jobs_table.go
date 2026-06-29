package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0006_06_14_000003_create_backup_jobs_table",
		Name:      "Create backup_jobs table",
		Timestamp: time.Date(2006, 6, 14, 0, 0, 3, 0, time.UTC),
		Up:        createBackupJobsTableUp,
	})
}

// backupJobMigration model for migration
type backupJobMigration struct {
	ID                string     `gorm:"type:char(26);primaryKey"`
	Status            string     `gorm:"type:varchar(255);not null"`
	BackupID          string     `gorm:"column:backup_id;type:char(26);not null;index"`
	StorageProviderID uint64     `gorm:"column:storage_provider_id;not null;index"`
	Size              *int       `gorm:"type:int"`
	Error             *string    `gorm:"type:text"`
	CreatedAt         *time.Time `gorm:"type:timestamp null"`
	UpdatedAt         *time.Time `gorm:"type:timestamp null"`
}

func (backupJobMigration) TableName() string {
	return "backup_jobs"
}

// backupJobWithBackupFK defines the backup foreign key
type backupJobWithBackupFK struct {
	BackupID string           `gorm:"column:backup_id"`
	Backup   *backupMigration `gorm:"foreignKey:BackupID;references:ID;constraint:OnDelete:CASCADE"`
}

func (backupJobWithBackupFK) TableName() string {
	return "backup_jobs"
}

// backupJobWithStorageProviderFK defines the storage provider foreign key
type backupJobWithStorageProviderFK struct {
	StorageProviderID uint64                    `gorm:"column:storage_provider_id"`
	StorageProvider   *storageProviderMigration `gorm:"foreignKey:StorageProviderID;references:ID;constraint:OnDelete:CASCADE"`
}

func (backupJobWithStorageProviderFK) TableName() string {
	return "backup_jobs"
}

func createBackupJobsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&backupJobMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&backupJobWithBackupFK{}, "Backup"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&backupJobWithStorageProviderFK{}, "StorageProvider")
}
