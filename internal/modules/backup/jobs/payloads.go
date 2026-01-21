package jobs

// BackupJobPayload is the common payload for backup-related jobs
// that operate on a specific backup.
type BackupJobPayload struct {
	ServerID string  `json:"server_id"`
	BackupID string  `json:"backup_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// Type aliases for clarity
type InstallBackupPayload = BackupJobPayload
type RunManualBackupPayload = BackupJobPayload
