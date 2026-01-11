package backup

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

// StorageDriver represents the type of storage provider
type StorageDriver string

const (
	StorageDriverS3      StorageDriver = "s3"
	StorageDriverDropbox StorageDriver = "dropbox"
)

// String returns the string representation of StorageDriver
func (d StorageDriver) String() string {
	return string(d)
}

// IsValid checks if the driver is a valid StorageDriver
func (d StorageDriver) IsValid() bool {
	switch d {
	case StorageDriverS3, StorageDriverDropbox:
		return true
	}
	return false
}

// Label returns the human-readable label for the storage driver
func (d StorageDriver) Label() string {
	switch d {
	case StorageDriverS3:
		return "S3"
	case StorageDriverDropbox:
		return "Dropbox"
	default:
		return ""
	}
}

// AllStorageDrivers returns all valid storage drivers
func AllStorageDrivers() []StorageDriver {
	return []StorageDriver{
		StorageDriverS3,
		StorageDriverDropbox,
	}
}

// StorageDriverValues returns all storage driver string values
func StorageDriverValues() []string {
	drivers := AllStorageDrivers()
	values := make([]string, len(drivers))
	for i, d := range drivers {
		values[i] = string(d)
	}
	return values
}
