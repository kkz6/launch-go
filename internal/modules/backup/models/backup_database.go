package models

// BackupDatabase represents the many-to-many relationship between backups and databases
type BackupDatabase struct {
	BackupID   string `gorm:"column:backup_id;type:char(26);not null;uniqueIndex:idx_backup_database" json:"backup_id"`
	DatabaseID string `gorm:"column:database_id;type:char(26);not null;uniqueIndex:idx_backup_database" json:"database_id"`
}

// TableName returns the table name for BackupDatabase
func (BackupDatabase) TableName() string {
	return "backup_databases"
}
