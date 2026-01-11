package backup

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/backup/enums"
)

func TestBackupJobStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status enums.BackupJobStatus
		want   string
	}{
		{"pending", enums.BackupJobStatusPending, "pending"},
		{"running", enums.BackupJobStatusRunning, "running"},
		{"finished", enums.BackupJobStatusFinished, "finished"},
		{"failed", enums.BackupJobStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("BackupJobStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackupJobStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status enums.BackupJobStatus
		want   bool
	}{
		{"pending is valid", enums.BackupJobStatusPending, true},
		{"running is valid", enums.BackupJobStatusRunning, true},
		{"finished is valid", enums.BackupJobStatusFinished, true},
		{"failed is valid", enums.BackupJobStatusFailed, true},
		{"invalid status", enums.BackupJobStatus("invalid"), false},
		{"empty status", enums.BackupJobStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("BackupJobStatus.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllBackupJobStatuses(t *testing.T) {
	statuses := enums.AllBackupJobStatuses()

	if len(statuses) != 4 {
		t.Errorf("AllBackupJobStatuses() returned %d statuses, want 4", len(statuses))
	}

	expected := []enums.BackupJobStatus{
		enums.BackupJobStatusPending,
		enums.BackupJobStatusRunning,
		enums.BackupJobStatusFinished,
		enums.BackupJobStatusFailed,
	}

	for i, status := range expected {
		if statuses[i] != status {
			t.Errorf("AllBackupJobStatuses()[%d] = %v, want %v", i, statuses[i], status)
		}
	}
}

func TestStorageDriver_String(t *testing.T) {
	tests := []struct {
		name   string
		driver enums.StorageDriver
		want   string
	}{
		{"s3", enums.StorageDriverS3, "s3"},
		{"dropbox", enums.StorageDriverDropbox, "dropbox"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.driver.String(); got != tt.want {
				t.Errorf("StorageDriver.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorageDriver_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		driver enums.StorageDriver
		want   bool
	}{
		{"s3 is valid", enums.StorageDriverS3, true},
		{"dropbox is valid", enums.StorageDriverDropbox, true},
		{"invalid driver", enums.StorageDriver("invalid"), false},
		{"empty driver", enums.StorageDriver(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.driver.IsValid(); got != tt.want {
				t.Errorf("StorageDriver.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorageDriver_Label(t *testing.T) {
	tests := []struct {
		name   string
		driver enums.StorageDriver
		want   string
	}{
		{"s3 label", enums.StorageDriverS3, "S3"},
		{"dropbox label", enums.StorageDriverDropbox, "Dropbox"},
		{"invalid driver label", enums.StorageDriver("invalid"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.driver.Label(); got != tt.want {
				t.Errorf("StorageDriver.Label() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllStorageDrivers(t *testing.T) {
	drivers := enums.AllStorageDrivers()

	if len(drivers) != 2 {
		t.Errorf("AllStorageDrivers() returned %d drivers, want 2", len(drivers))
	}

	expected := []enums.StorageDriver{
		enums.StorageDriverS3,
		enums.StorageDriverDropbox,
	}

	for i, driver := range expected {
		if drivers[i] != driver {
			t.Errorf("AllStorageDrivers()[%d] = %v, want %v", i, drivers[i], driver)
		}
	}
}

func TestStorageDriverValues(t *testing.T) {
	values := enums.StorageDriverValues()

	if len(values) != 2 {
		t.Errorf("StorageDriverValues() returned %d values, want 2", len(values))
	}

	expected := []string{"s3", "dropbox"}

	for i, value := range expected {
		if values[i] != value {
			t.Errorf("StorageDriverValues()[%d] = %v, want %v", i, values[i], value)
		}
	}
}
