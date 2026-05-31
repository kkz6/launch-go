package dto

import (
	"encoding/json"
	"strconv"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// BackupResponse represents the response for a backup
type BackupResponse struct {
	ID                    string              `json:"id"`
	ServerID              string              `json:"server_id"`
	UserID                string              `json:"user_id"`
	StorageProviderID     string              `json:"storage_provider_id"`
	CronExpression        string              `json:"cron_expression"`
	IncludeFiles          []string            `json:"include_files"`
	ExcludeFiles          []string            `json:"exclude_files"`
	Retention             int                 `json:"retention"`
	NotificationOnFailure bool                `json:"notification_on_failure"`
	NotificationOnSuccess bool                `json:"notification_on_success"`
	Enabled               bool                `json:"enabled"`
	Path                  string              `json:"path"`
	InstalledAt           *string             `json:"installed_at,omitempty"`
	InstallationFailedAt  *string             `json:"installation_failed_at,omitempty"`
	SizeInMB              int64               `json:"size_in_mb"`
	CreatedAt             string              `json:"created_at"`
	UpdatedAt             string              `json:"updated_at"`
	Jobs                  []BackupJobResponse `json:"jobs,omitempty"`
	Databases             []string            `json:"databases,omitempty"`
	LatestJob             *BackupJobResponse  `json:"latest_job,omitempty"`
}

// ToBackupResponse converts a Backup model to BackupResponse
func ToBackupResponse(backup *models.Backup) BackupResponse {
	userID := ""
	if backup.UserID != nil {
		userID = *backup.UserID
	}

	resp := BackupResponse{
		ID:                    backup.ID,
		ServerID:              backup.ServerID,
		UserID:                userID,
		StorageProviderID:     strconv.FormatUint(backup.StorageProviderID, 10),
		CronExpression:        backup.CronExpression,
		Retention:             backup.Retention,
		NotificationOnFailure: backup.NotificationOnFailure,
		NotificationOnSuccess: backup.NotificationOnSuccess,
		Enabled:               backup.Enabled,
		Path:                  backup.Path,
		SizeInMB:              backup.GetSizeInMB(),
		InstalledAt:           pkgdto.FormatTime(backup.InstalledAt),
		InstallationFailedAt:  pkgdto.FormatTime(backup.InstallationFailedAt),
		CreatedAt:             pkgdto.FormatTimeOrEmpty(backup.CreatedAt),
		UpdatedAt:             pkgdto.FormatTimeOrEmpty(backup.UpdatedAt),
	}

	// Handle include/exclude files (JSON strings)
	var includeFiles []string
	if err := json.Unmarshal([]byte(backup.IncludeFiles), &includeFiles); err == nil {
		resp.IncludeFiles = includeFiles
	} else {
		resp.IncludeFiles = []string{}
	}

	var excludeFiles []string
	if err := json.Unmarshal([]byte(backup.ExcludeFiles), &excludeFiles); err == nil {
		resp.ExcludeFiles = excludeFiles
	} else {
		resp.ExcludeFiles = []string{}
	}

	// Convert jobs
	if len(backup.Jobs) > 0 {
		resp.Jobs = pkgdto.TransformSlice(backup.Jobs, ToBackupJobResponse)
		// Set latest job (assuming jobs are ordered by created_at desc)
		latestJob := ToBackupJobResponse(&backup.Jobs[0])
		resp.LatestJob = &latestJob
	}

	// Convert database IDs
	if len(backup.Databases) > 0 {
		resp.Databases = pkgdto.MapSlice(backup.Databases, func(db *models.BackupDatabase) string {
			return db.DatabaseID
		})
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
	// TaskID drives the live log console: the UI streams the run's
	// tar/upload output via ServerLogViewer entity="task".
	TaskID    *string `json:"task_id,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ToBackupJobResponse converts a BackupJob model to BackupJobResponse
func ToBackupJobResponse(job *models.BackupJob) BackupJobResponse {
	var size int64
	if job.Size != nil {
		size = int64(*job.Size)
	}

	resp := BackupJobResponse{
		ID:                job.ID,
		BackupID:          job.BackupID,
		StorageProviderID: strconv.FormatUint(job.StorageProviderID, 10),
		Status:            string(job.Status),
		Size:              size,
		SizeInMB:          job.GetSizeInMB(),
		TaskID:            job.TaskID,
		CreatedAt:         pkgdto.FormatTimeOrEmpty(job.CreatedAt),
		UpdatedAt:         pkgdto.FormatTimeOrEmpty(job.UpdatedAt),
	}

	if job.Error != nil {
		resp.Error = *job.Error
	}

	return resp
}

// StorageProviderResponse represents the response for a storage provider
type StorageProviderResponse struct {
	ID             uint64  `json:"id"`
	UserID         string  `json:"user_id"`
	TeamID         string  `json:"team_id"`
	Provider       string  `json:"provider"`
	ProviderLabel  string  `json:"provider_label"`
	Label          string  `json:"label"`
	Connected      bool    `json:"connected"`
	TokenExpiresAt *string `json:"token_expires_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// ToStorageProviderResponse converts a StorageProvider model to StorageProviderResponse
func ToStorageProviderResponse(provider *models.StorageProvider) StorageProviderResponse {
	label := ""
	if provider.Label != nil {
		label = *provider.Label
	}

	return StorageProviderResponse{
		ID:             provider.ID,
		UserID:         provider.UserID,
		TeamID:         provider.TeamID,
		Provider:       string(provider.Provider),
		ProviderLabel:  provider.Provider.Label(),
		Label:          label,
		Connected:      provider.Connected,
		TokenExpiresAt: pkgdto.FormatTime(provider.TokenExpiresAt),
		CreatedAt:      pkgdto.FormatTimeOrEmpty(provider.CreatedAt),
		UpdatedAt:      pkgdto.FormatTimeOrEmpty(provider.UpdatedAt),
	}
}

// StorageProviderListItem represents a simplified storage provider for list/dropdown
type StorageProviderListItem struct {
	ID    uint64 `json:"id"`
	Label string `json:"label"`
}

// ToStorageProviderListItem converts a StorageProvider to a list item
func ToStorageProviderListItem(provider *models.StorageProvider) StorageProviderListItem {
	label := ""
	if provider.Label != nil {
		label = *provider.Label
	}

	return StorageProviderListItem{
		ID:    provider.ID,
		Label: label,
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
