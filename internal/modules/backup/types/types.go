// Package types contains all type definitions for the backup module
package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// BackupJobStatus
// =============================================================================

// BackupJobStatus represents the status of a backup job
type BackupJobStatus string

const (
	BackupJobStatusPending  BackupJobStatus = "pending"
	BackupJobStatusRunning  BackupJobStatus = "running"
	BackupJobStatusFinished BackupJobStatus = "finished"
	BackupJobStatusFailed   BackupJobStatus = "failed"
)

var allBackupJobStatuses = []BackupJobStatus{
	BackupJobStatusPending,
	BackupJobStatusRunning,
	BackupJobStatusFinished,
	BackupJobStatusFailed,
}

// AllBackupJobStatuses returns all valid backup job statuses
func AllBackupJobStatuses() []BackupJobStatus {
	return allBackupJobStatuses
}

// String returns the string representation of BackupJobStatus
func (s BackupJobStatus) String() string {
	return string(s)
}

// Label returns a human-readable label for the backup job status
func (s BackupJobStatus) Label() string {
	switch s {
	case BackupJobStatusPending:
		return "Pending"
	case BackupJobStatusRunning:
		return "Running"
	case BackupJobStatusFinished:
		return "Finished"
	case BackupJobStatusFailed:
		return "Failed"
	default:
		return string(s)
	}
}

// IsValid checks if the status is a valid BackupJobStatus
func (s BackupJobStatus) IsValid() bool {
	return enumtypes.IsValid(s, allBackupJobStatuses...)
}

// Value implements driver.Valuer for database storage
func (s BackupJobStatus) Value() (driver.Value, error) {
	return enumtypes.Value(s)
}

// Scan implements sql.Scanner for database retrieval
func (s *BackupJobStatus) Scan(value any) error {
	return enumtypes.Scan(s, value)
}

// =============================================================================
// StorageDriver
// =============================================================================

// StorageDriver represents the type of storage provider
type StorageDriver string

const (
	StorageDriverS3      StorageDriver = "s3"
	StorageDriverDropbox StorageDriver = "dropbox"
)

var allStorageDrivers = []StorageDriver{
	StorageDriverS3,
	StorageDriverDropbox,
}

// AllStorageDrivers returns all valid storage drivers
func AllStorageDrivers() []StorageDriver {
	return allStorageDrivers
}

// String returns the string representation of StorageDriver
func (d StorageDriver) String() string {
	return string(d)
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

// IsValid checks if the driver is a valid StorageDriver
func (d StorageDriver) IsValid() bool {
	return enumtypes.IsValid(d, allStorageDrivers...)
}

// Value implements driver.Valuer for database storage
func (d StorageDriver) Value() (driver.Value, error) {
	return enumtypes.Value(d)
}

// Scan implements sql.Scanner for database retrieval
func (d *StorageDriver) Scan(value any) error {
	return enumtypes.Scan(d, value)
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
