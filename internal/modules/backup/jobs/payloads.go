package jobs

// BackupJobPayload is the common payload for backup-related jobs
// that operate on a specific backup.
type BackupJobPayload struct {
	ServerID string  `json:"server_id"`
	BackupID string  `json:"backup_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallBackupPayload is the payload for backup installation jobs.
type InstallBackupPayload = BackupJobPayload

// RunManualBackupPayload identifies a manual backup run.
type RunManualBackupPayload struct {
	ServerID string  `json:"server_id"`
	BackupID string  `json:"backup_id"`
	TeamID   string  `json:"team_id,omitempty"`
	JobID    string  `json:"job_id,omitempty"`
	UserID   *string `json:"user_id,omitempty"`
}
