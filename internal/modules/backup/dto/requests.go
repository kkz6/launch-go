package dto

import (
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
)

// CreateBackupRequest represents a request to create a backup configuration
type CreateBackupRequest struct {
	CronExpression        string   `json:"cron_expression" validate:"required"`
	Path                  string   `json:"path" validate:"required"`
	Enabled               bool     `json:"enabled"`
	DatabaseID            string   `json:"database" validate:"required"`
	StorageProviderID     string   `json:"storage_provider_id" validate:"required"`
	IncludeFiles          []string `json:"include_files,omitempty"`
	ExcludeFiles          []string `json:"exclude_files,omitempty"`
	Retention             int      `json:"retention,omitempty" validate:"omitempty,min=1,max=365"`
	NotificationOnFailure bool     `json:"notification_on_failure,omitempty"`
	NotificationOnSuccess bool     `json:"notification_on_success,omitempty"`
}

// UpdateBackupRequest represents a request to update a backup configuration
type UpdateBackupRequest struct {
	CronExpression        string   `json:"cron_expression" validate:"required"`
	Path                  string   `json:"path" validate:"required"`
	Enabled               bool     `json:"enabled"`
	DatabaseID            string   `json:"database" validate:"required"`
	StorageProviderID     string   `json:"storage_provider_id" validate:"required"`
	IncludeFiles          []string `json:"include_files,omitempty"`
	ExcludeFiles          []string `json:"exclude_files,omitempty"`
	Retention             int      `json:"retention,omitempty" validate:"omitempty,min=1,max=365"`
	NotificationOnFailure bool     `json:"notification_on_failure,omitempty"`
	NotificationOnSuccess bool     `json:"notification_on_success,omitempty"`
}

// CreateBackupJobRequest represents a request to create a backup job (webhook from agent)
type CreateBackupJobRequest struct {
	Status backuptypes.BackupJobStatus `json:"status" validate:"required,oneof=pending running finished failed"`
	Size   int64                       `json:"size,omitempty"`
	Error  string                      `json:"error,omitempty"`
}

// CreateStorageProviderRequest represents a request to create a storage provider
type CreateStorageProviderRequest struct {
	Label    string `json:"label" validate:"required,max=255"`
	Provider string `json:"provider" validate:"required,oneof=s3 dropbox"`

	// S3-specific fields
	Endpoint       string `json:"endpoint,omitempty"`
	Key            string `json:"key,omitempty" validate:"required_if=Provider s3"`
	Secret         string `json:"secret,omitempty" validate:"required_if=Provider s3"`
	Region         string `json:"region,omitempty" validate:"required_if=Provider s3"`
	Bucket         string `json:"bucket,omitempty" validate:"required_if=Provider s3"`
	Path           string `json:"path,omitempty"`
	ForcePathStyle bool   `json:"force_path_style,omitempty"`

	// Dropbox-specific fields
	Token string `json:"token,omitempty" validate:"required_if=Provider dropbox"`
}

// UpdateStorageProviderRequest represents a request to update a storage provider
type UpdateStorageProviderRequest struct {
	ID       uint64 `json:"id" validate:"required"`
	Label    string `json:"label" validate:"required,max=255"`
	Provider string `json:"provider" validate:"required,oneof=s3 dropbox"`

	// S3-specific fields
	Endpoint       string `json:"endpoint,omitempty"`
	Key            string `json:"key,omitempty" validate:"required_if=Provider s3"`
	Secret         string `json:"secret,omitempty" validate:"required_if=Provider s3"`
	Region         string `json:"region,omitempty" validate:"required_if=Provider s3"`
	Bucket         string `json:"bucket,omitempty" validate:"required_if=Provider s3"`
	Path           string `json:"path,omitempty"`
	ForcePathStyle bool   `json:"force_path_style,omitempty"`

	// Dropbox-specific fields
	Token string `json:"token,omitempty" validate:"required_if=Provider dropbox"`
}
