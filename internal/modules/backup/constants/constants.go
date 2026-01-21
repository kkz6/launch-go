package constants

import "time"

// Backup types
const (
	TypeDatabase = "database"
	TypeFiles    = "files"
	TypeFull     = "full"
)

// AllBackupTypes returns all supported backup types
var AllBackupTypes = []string{TypeDatabase, TypeFiles, TypeFull}

// Backup statuses
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// Storage providers
const (
	ProviderS3        = "s3"
	ProviderSpaces    = "spaces"
	ProviderBackblaze = "backblaze"
	ProviderWasabi    = "wasabi"
	ProviderLocal     = "local"
)

// AllStorageProviders returns all supported storage providers
var AllStorageProviders = []string{
	ProviderS3,
	ProviderSpaces,
	ProviderBackblaze,
	ProviderWasabi,
	ProviderLocal,
}

// Default settings
const (
	DefaultRetentionDays    = 7
	DefaultCompressionLevel = 6
	MaxRetentionDays        = 365
)

// Schedule presets
const (
	ScheduleHourly  = "hourly"
	ScheduleDaily   = "daily"
	ScheduleWeekly  = "weekly"
	ScheduleMonthly = "monthly"
)

// Timeout settings
const (
	BackupTimeout  = 2 * time.Hour
	RestoreTimeout = 4 * time.Hour
	UploadTimeout  = 30 * time.Minute
)

// Size limits
const (
	MaxBackupSizeGB = 100
	ChunkSizeMB     = 100
)
