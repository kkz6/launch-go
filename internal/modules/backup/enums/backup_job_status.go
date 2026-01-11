package enums

// BackupJobStatus represents the status of a backup job
type BackupJobStatus string

const (
	BackupJobStatusPending  BackupJobStatus = "pending"
	BackupJobStatusRunning  BackupJobStatus = "running"
	BackupJobStatusFinished BackupJobStatus = "finished"
	BackupJobStatusFailed   BackupJobStatus = "failed"
)

// String returns the string representation of BackupJobStatus
func (s BackupJobStatus) String() string {
	return string(s)
}

// IsValid checks if the status is a valid BackupJobStatus
func (s BackupJobStatus) IsValid() bool {
	switch s {
	case BackupJobStatusPending, BackupJobStatusRunning, BackupJobStatusFinished, BackupJobStatusFailed:
		return true
	}

	return false
}

// AllBackupJobStatuses returns all valid backup job statuses
func AllBackupJobStatuses() []BackupJobStatus {
	return []BackupJobStatus{
		BackupJobStatusPending,
		BackupJobStatusRunning,
		BackupJobStatusFinished,
		BackupJobStatusFailed,
	}
}
