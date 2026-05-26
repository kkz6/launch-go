package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0024_05_22_000003_create_docker_database_backups_tables",
		Name:      "Create docker database backup tables",
		Timestamp: time.Date(2024, 5, 22, 0, 0, 3, 0, time.UTC),
		Up:        createDockerDatabaseBackupTablesUp,
		Down:      createDockerDatabaseBackupTablesDown,
	})
}

// docker_database_backups — one row per database describing where + how
// often to upload snapshots. The "where" (S3 bucket + creds) is stored
// encrypted because the row also carries the access key.
//
// docker_database_backup_runs — one row per past run. Lets the UI show
// "last successful backup on …" and lets us restore from a specific
// snapshot ("restore database X from backup Y from 2026-04-12 03:00").
type dockerDatabaseBackupMigration struct {
	ID           string         `gorm:"type:char(26);primaryKey"`
	TeamID       string         `gorm:"column:team_id;type:char(26);not null;index"`
	DatabaseID   string         `gorm:"column:database_id;type:char(26);not null;index"`
	Provider     string         `gorm:"type:varchar(32);not null;default:s3"`
	Endpoint     *string        `gorm:"type:varchar(255)"`
	Bucket       string         `gorm:"type:varchar(255);not null"`
	Region       *string        `gorm:"type:varchar(64)"`
	PathPrefix   *string        `gorm:"column:path_prefix;type:varchar(255)"`
	Credentials  *string        `gorm:"type:longtext"` // encrypted JSON
	CronSchedule *string        `gorm:"column:cron_schedule;type:varchar(64)"`
	Enabled      bool           `gorm:"not null;default:true"`
	CreatedAt    *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt    *time.Time     `gorm:"type:timestamp null"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (dockerDatabaseBackupMigration) TableName() string { return "docker_database_backups" }

type dockerDatabaseBackupWithDatabaseFK struct {
	DatabaseID string                   `gorm:"column:database_id"`
	Database   *dockerDatabaseMigration `gorm:"foreignKey:DatabaseID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerDatabaseBackupWithDatabaseFK) TableName() string { return "docker_database_backups" }

type dockerDatabaseBackupRunMigration struct {
	ID         string     `gorm:"type:char(26);primaryKey"`
	BackupID   string     `gorm:"column:backup_id;type:char(26);not null;index"`
	Status     string     `gorm:"type:varchar(32);not null;default:running"`
	StartedAt  *time.Time `gorm:"column:started_at;type:timestamp null"`
	FinishedAt *time.Time `gorm:"column:finished_at;type:timestamp null"`
	ObjectKey  *string    `gorm:"column:object_key;type:varchar(512)"`
	SizeBytes  *int64     `gorm:"column:size_bytes"`
	Error      *string    `gorm:"type:text"`
	CreatedAt  *time.Time `gorm:"type:timestamp null"`
	UpdatedAt  *time.Time `gorm:"type:timestamp null"`
}

func (dockerDatabaseBackupRunMigration) TableName() string { return "docker_database_backup_runs" }

type dockerDatabaseBackupRunWithBackupFK struct {
	BackupID string                         `gorm:"column:backup_id"`
	Backup   *dockerDatabaseBackupMigration `gorm:"foreignKey:BackupID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerDatabaseBackupRunWithBackupFK) TableName() string { return "docker_database_backup_runs" }

func createDockerDatabaseBackupTablesUp(db *gorm.DB) error {
	migrator := db.Set(
		"gorm:table_options",
		"DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
	).Migrator()

	if err := migrator.CreateTable(&dockerDatabaseBackupMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateTable(&dockerDatabaseBackupRunMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerDatabaseBackupWithDatabaseFK{}, "Database"); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerDatabaseBackupRunWithBackupFK{}, "Backup"); err != nil {
		return err
	}
	// One backup config per database (live). Soft-deleted rows can
	// coexist so re-configuring a removed backup works.
	return db.Exec(
		"CREATE UNIQUE INDEX idx_docker_db_backups_db ON docker_database_backups (database_id, deleted_at)",
	).Error
}

func createDockerDatabaseBackupTablesDown(db *gorm.DB) error {
	if err := db.Migrator().DropTable(&dockerDatabaseBackupRunMigration{}); err != nil {
		return err
	}
	return db.Migrator().DropTable(&dockerDatabaseBackupMigration{})
}
