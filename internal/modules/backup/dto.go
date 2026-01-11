package backup

import (
	"time"
)

// CreateBackupRequest represents a request to create a backup configuration
type CreateBackupRequest struct {
	CronExpression      string   `json:"cron_expression" validate:"required"`
	Path                string   `json:"path" validate:"required"`
	Enabled             bool     `json:"enabled"`
	DatabaseID          string   `json:"database" validate:"required"`
	StorageProviderID   string   `json:"storage_provider_id" validate:"required"`
	IncludeFiles        []string `json:"include_files,omitempty"`
	ExcludeFiles        []string `json:"exclude_files,omitempty"`
	Retention           int      `json:"retention,omitempty" validate:"omitempty,min=1,max=365"`
	NotificationOnFailure bool   `json:"notification_on_failure,omitempty"`
	NotificationOnSuccess bool   `json:"notification_on_success,omitempty"`
}

// UpdateBackupRequest represents a request to update a backup configuration
type UpdateBackupRequest struct {
	CronExpression      string   `json:"cron_expression" validate:"required"`
	Path                string   `json:"path" validate:"required"`
	Enabled             bool     `json:"enabled"`
	DatabaseID          string   `json:"database" validate:"required"`
	StorageProviderID   string   `json:"storage_provider_id" validate:"required"`
	IncludeFiles        []string `json:"include_files,omitempty"`
	ExcludeFiles        []string `json:"exclude_files,omitempty"`
	Retention           int      `json:"retention,omitempty" validate:"omitempty,min=1,max=365"`
	NotificationOnFailure bool   `json:"notification_on_failure,omitempty"`
	NotificationOnSuccess bool   `json:"notification_on_success,omitempty"`
}

// CreateBackupJobRequest represents a request to create a backup job (webhook from agent)
type CreateBackupJobRequest struct {
	Status BackupJobStatus `json:"status" validate:"required,oneof=pending running finished failed"`
	Size   int64           `json:"size,omitempty"`
	Error  string          `json:"error,omitempty"`
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
	ID       uint   `json:"id" validate:"required"`
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

// BackupResponse represents the response for a backup
type BackupResponse struct {
	ID                    string             `json:"id"`
	ServerID              string             `json:"server_id"`
	UserID                string             `json:"user_id"`
	StorageProviderID     string             `json:"storage_provider_id"`
	CronExpression        string             `json:"cron_expression"`
	IncludeFiles          []string           `json:"include_files"`
	ExcludeFiles          []string           `json:"exclude_files"`
	Retention             int                `json:"retention"`
	NotificationOnFailure bool               `json:"notification_on_failure"`
	NotificationOnSuccess bool               `json:"notification_on_success"`
	Enabled               bool               `json:"enabled"`
	Path                  string             `json:"path"`
	InstalledAt           *string            `json:"installed_at,omitempty"`
	InstallationFailedAt  *string            `json:"installation_failed_at,omitempty"`
	SizeInMB              int64              `json:"size_in_mb"`
	CreatedAt             string             `json:"created_at"`
	UpdatedAt             string             `json:"updated_at"`
	Jobs                  []BackupJobResponse `json:"jobs,omitempty"`
	Databases             []string           `json:"databases,omitempty"`
	LatestJob             *BackupJobResponse `json:"latest_job,omitempty"`
}

// ToBackupResponse converts a Backup model to BackupResponse
func ToBackupResponse(backup *Backup) BackupResponse {
	resp := BackupResponse{
		ID:                    backup.ID,
		ServerID:              backup.ServerID,
		UserID:                backup.UserID,
		StorageProviderID:     backup.StorageProviderID,
		CronExpression:        backup.CronExpression,
		Retention:             backup.Retention,
		NotificationOnFailure: backup.NotificationOnFailure,
		NotificationOnSuccess: backup.NotificationOnSuccess,
		Enabled:               backup.Enabled,
		Path:                  backup.Path,
		SizeInMB:              backup.GetSizeInMB(),
		CreatedAt:             backup.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             backup.UpdatedAt.Format(time.RFC3339),
	}

	// Handle include/exclude files
	if includeFiles, err := backup.IncludeFiles.ToStringSlice(); err == nil {
		resp.IncludeFiles = includeFiles
	} else {
		resp.IncludeFiles = []string{}
	}

	if excludeFiles, err := backup.ExcludeFiles.ToStringSlice(); err == nil {
		resp.ExcludeFiles = excludeFiles
	} else {
		resp.ExcludeFiles = []string{}
	}

	if backup.InstalledAt != nil {
		installedAt := backup.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &installedAt
	}

	if backup.InstallationFailedAt != nil {
		failedAt := backup.InstallationFailedAt.Format(time.RFC3339)
		resp.InstallationFailedAt = &failedAt
	}

	// Convert jobs
	if len(backup.Jobs) > 0 {
		resp.Jobs = make([]BackupJobResponse, len(backup.Jobs))
		for i, job := range backup.Jobs {
			resp.Jobs[i] = ToBackupJobResponse(&job)
		}
		// Set latest job (assuming jobs are ordered by created_at desc)
		latestJob := ToBackupJobResponse(&backup.Jobs[0])
		resp.LatestJob = &latestJob
	}

	// Convert database IDs
	if len(backup.Databases) > 0 {
		resp.Databases = make([]string, len(backup.Databases))
		for i, db := range backup.Databases {
			resp.Databases[i] = db.DatabaseID
		}
	} else {
		resp.Databases = []string{}
	}

	return resp
}

// BackupJobResponse represents the response for a backup job
type BackupJobResponse struct {
	ID                string `json:"id"`
	BackupID          string `json:"backup_id"`
	StorageProviderID string `json:"storage_provider_id"`
	Status            string `json:"status"`
	Size              int64  `json:"size"`
	SizeInMB          int64  `json:"size_in_mb"`
	Error             string `json:"error,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// ToBackupJobResponse converts a BackupJob model to BackupJobResponse
func ToBackupJobResponse(job *BackupJob) BackupJobResponse {
	resp := BackupJobResponse{
		ID:                job.ID,
		BackupID:          job.BackupID,
		StorageProviderID: job.StorageProviderID,
		Status:            string(job.Status),
		Size:              job.Size,
		SizeInMB:          job.GetSizeInMB(),
		CreatedAt:         job.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         job.UpdatedAt.Format(time.RFC3339),
	}

	if job.Error != nil {
		resp.Error = *job.Error
	}

	return resp
}

// StorageProviderResponse represents the response for a storage provider
type StorageProviderResponse struct {
	ID             uint   `json:"id"`
	UserID         string `json:"user_id"`
	TeamID         string `json:"team_id"`
	Provider       string `json:"provider"`
	ProviderLabel  string `json:"provider_label"`
	Label          string `json:"label"`
	Connected      bool   `json:"connected"`
	TokenExpiresAt *string `json:"token_expires_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// ToStorageProviderResponse converts a StorageProvider model to StorageProviderResponse
func ToStorageProviderResponse(provider *StorageProvider) StorageProviderResponse {
	resp := StorageProviderResponse{
		ID:            provider.ID,
		UserID:        provider.UserID,
		TeamID:        provider.TeamID,
		Provider:      string(provider.Provider),
		ProviderLabel: provider.Provider.Label(),
		Label:         provider.Label,
		Connected:     provider.Connected,
		CreatedAt:     provider.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     provider.UpdatedAt.Format(time.RFC3339),
	}

	if provider.TokenExpiresAt != nil {
		expiresAt := provider.TokenExpiresAt.Format(time.RFC3339)
		resp.TokenExpiresAt = &expiresAt
	}

	return resp
}

// StorageProviderListItem represents a simplified storage provider for list/dropdown
type StorageProviderListItem struct {
	ID    uint   `json:"id"`
	Label string `json:"label"`
}

// ToStorageProviderListItem converts a StorageProvider to a list item
func ToStorageProviderListItem(provider *StorageProvider) StorageProviderListItem {
	return StorageProviderListItem{
		ID:    provider.ID,
		Label: provider.Label,
	}
}

// AgentBackupConfig represents the backup configuration for the agent
type AgentBackupConfig struct {
	ID             string                 `json:"id"`
	CronExpression string                 `json:"cron_expression"`
	Path           string                 `json:"path"`
	Retention      int                    `json:"retention"`
	WebhookURL     string                 `json:"webhook_url"`
	IncludeFiles   []string               `json:"include_files"`
	ExcludeFiles   []string               `json:"exclude_files"`
	Databases      []string               `json:"databases"`
	Storage        map[string]interface{} `json:"storage"`
	StorageDriver  string                 `json:"storage_driver"`
}
