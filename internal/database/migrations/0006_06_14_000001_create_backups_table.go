package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0006_06_14_000001_create_backups_table",
		Name:      "Create backups table",
		Timestamp: time.Date(2006, 6, 14, 0, 0, 1, 0, time.UTC),
		Up:        createBackupsTableUp,
	})
}

// backupMigration model for migration
type backupMigration struct {
	ID                        string     `gorm:"type:char(26);primaryKey"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index"`
	UserID                    *string    `gorm:"column:user_id;type:char(26);index"`
	StorageProviderID         uint64     `gorm:"column:storage_provider_id;not null;index"`
	DispatchToken             string     `gorm:"column:dispatch_token;type:varchar(32);not null"`
	CronExpression            string     `gorm:"column:cron_expression;type:varchar(255);not null"`
	IncludeFiles              string     `gorm:"column:include_files;type:jsonb;not null"`
	ExcludeFiles              string     `gorm:"column:exclude_files;type:jsonb;not null"`
	Retention                 int        `gorm:"default:14"`
	NotificationOnFailure     bool       `gorm:"column:notification_on_failure;default:true"`
	NotificationOnSuccess     bool       `gorm:"column:notification_on_success;default:true"`
	Enabled                   bool       `gorm:"default:false"`
	Path                      string     `gorm:"type:varchar(255);not null"`
	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null"`
}

func (backupMigration) TableName() string {
	return "backups"
}

// backupWithServerFK defines the server foreign key
type backupWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (backupWithServerFK) TableName() string {
	return "backups"
}

// backupWithUserFK defines the user foreign key (nullable, null on delete)
type backupWithUserFK struct {
	UserID *string        `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:SET NULL"`
}

func (backupWithUserFK) TableName() string {
	return "backups"
}

// backupWithStorageProviderFK defines the storage provider foreign key
type backupWithStorageProviderFK struct {
	StorageProviderID uint64                    `gorm:"column:storage_provider_id"`
	StorageProvider   *storageProviderMigration `gorm:"foreignKey:StorageProviderID;references:ID"`
}

func (backupWithStorageProviderFK) TableName() string {
	return "backups"
}

func createBackupsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&backupMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&backupWithServerFK{}, "Server"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&backupWithUserFK{}, "User"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&backupWithStorageProviderFK{}, "StorageProvider")
}
