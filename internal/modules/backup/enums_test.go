package backup

import (
	"testing"
)

func TestBackupJobStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status BackupJobStatus
		want   string
	}{
		{"pending", BackupJobStatusPending, "pending"},
		{"running", BackupJobStatusRunning, "running"},
		{"finished", BackupJobStatusFinished, "finished"},
		{"failed", BackupJobStatusFailed, "failed"},
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
		status BackupJobStatus
		want   bool
	}{
		{"pending is valid", BackupJobStatusPending, true},
		{"running is valid", BackupJobStatusRunning, true},
		{"finished is valid", BackupJobStatusFinished, true},
		{"failed is valid", BackupJobStatusFailed, true},
		{"invalid status", BackupJobStatus("invalid"), false},
		{"empty status", BackupJobStatus(""), false},
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
	statuses := AllBackupJobStatuses()

	if len(statuses) != 4 {
		t.Errorf("AllBackupJobStatuses() returned %d statuses, want 4", len(statuses))
	}

	expected := []BackupJobStatus{
		BackupJobStatusPending,
		BackupJobStatusRunning,
		BackupJobStatusFinished,
		BackupJobStatusFailed,
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
		driver StorageDriver
		want   string
	}{
		{"s3", StorageDriverS3, "s3"},
		{"dropbox", StorageDriverDropbox, "dropbox"},
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
		driver StorageDriver
		want   bool
	}{
		{"s3 is valid", StorageDriverS3, true},
		{"dropbox is valid", StorageDriverDropbox, true},
		{"invalid driver", StorageDriver("invalid"), false},
		{"empty driver", StorageDriver(""), false},
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
		driver StorageDriver
		want   string
	}{
		{"s3 label", StorageDriverS3, "S3"},
		{"dropbox label", StorageDriverDropbox, "Dropbox"},
		{"invalid driver label", StorageDriver("invalid"), ""},
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
	drivers := AllStorageDrivers()

	if len(drivers) != 2 {
		t.Errorf("AllStorageDrivers() returned %d drivers, want 2", len(drivers))
	}

	expected := []StorageDriver{
		StorageDriverS3,
		StorageDriverDropbox,
	}

	for i, driver := range expected {
		if drivers[i] != driver {
			t.Errorf("AllStorageDrivers()[%d] = %v, want %v", i, drivers[i], driver)
		}
	}
}

func TestStorageDriverValues(t *testing.T) {
	values := StorageDriverValues()

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
