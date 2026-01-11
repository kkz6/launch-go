package backup

import (
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/enums"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
)

func TestToBackupResponse(t *testing.T) {
	now := time.Now()
	installedAt := now.Add(-time.Hour)

	includeFiles, _ := models.FromStringSlice([]string{"/app", "/config"})
	excludeFiles, _ := models.FromStringSlice([]string{"/cache"})

	backup := &models.Backup{
		ID:                    "backup123",
		ServerID:              "server123",
		UserID:                "user123",
		StorageProviderID:     "provider123",
		CronExpression:        "0 0 * * *",
		IncludeFiles:          includeFiles,
		ExcludeFiles:          excludeFiles,
		Retention:             7,
		NotificationOnFailure: true,
		NotificationOnSuccess: false,
		Enabled:               true,
		Path:                  "/var/www",
		InstalledAt:           &installedAt,
		CreatedAt:             now,
		UpdatedAt:             now,
		Jobs: []models.BackupJob{
			{ID: "job1", Status: enums.BackupJobStatusFinished, Size: 1024 * 1024},
			{ID: "job2", Status: enums.BackupJobStatusFailed, Size: 512 * 1024},
		},
		Databases: []models.BackupDatabase{
			{DatabaseID: "db1"},
			{DatabaseID: "db2"},
		},
	}

	resp := dto.ToBackupResponse(backup)

	if resp.ID != "backup123" {
		t.Errorf("ID = %s, want backup123", resp.ID)
	}
	if resp.ServerID != "server123" {
		t.Errorf("ServerID = %s, want server123", resp.ServerID)
	}
	if resp.CronExpression != "0 0 * * *" {
		t.Errorf("CronExpression = %s, want 0 0 * * *", resp.CronExpression)
	}
	if resp.Retention != 7 {
		t.Errorf("Retention = %d, want 7", resp.Retention)
	}
	if !resp.NotificationOnFailure {
		t.Error("expected NotificationOnFailure to be true")
	}
	if resp.NotificationOnSuccess {
		t.Error("expected NotificationOnSuccess to be false")
	}
	if resp.InstalledAt == nil {
		t.Error("expected InstalledAt to be set")
	}
	if len(resp.IncludeFiles) != 2 {
		t.Errorf("IncludeFiles length = %d, want 2", len(resp.IncludeFiles))
	}
	if len(resp.ExcludeFiles) != 1 {
		t.Errorf("ExcludeFiles length = %d, want 1", len(resp.ExcludeFiles))
	}
	if len(resp.Jobs) != 2 {
		t.Errorf("Jobs length = %d, want 2", len(resp.Jobs))
	}
	if len(resp.Databases) != 2 {
		t.Errorf("Databases length = %d, want 2", len(resp.Databases))
	}
	if resp.LatestJob == nil {
		t.Error("expected LatestJob to be set")
	}
}

func TestToBackupResponse_Empty(t *testing.T) {
	backup := &models.Backup{
		ID:             "backup123",
		ServerID:       "server123",
		UserID:         "user123",
		CronExpression: "0 0 * * *",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	resp := dto.ToBackupResponse(backup)

	if resp.InstalledAt != nil {
		t.Error("expected InstalledAt to be nil")
	}
	if resp.InstallationFailedAt != nil {
		t.Error("expected InstallationFailedAt to be nil")
	}
	if len(resp.Jobs) != 0 {
		t.Errorf("Jobs length = %d, want 0", len(resp.Jobs))
	}
	if len(resp.Databases) != 0 {
		t.Errorf("Databases length = %d, want 0", len(resp.Databases))
	}
	if resp.LatestJob != nil {
		t.Error("expected LatestJob to be nil")
	}
}

func TestToBackupResponse_InstallationFailed(t *testing.T) {
	now := time.Now()
	failedAt := now.Add(-time.Hour)

	backup := &models.Backup{
		ID:                   "backup123",
		ServerID:             "server123",
		UserID:               "user123",
		CronExpression:       "0 0 * * *",
		InstallationFailedAt: &failedAt,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	resp := dto.ToBackupResponse(backup)

	if resp.InstallationFailedAt == nil {
		t.Error("expected InstallationFailedAt to be set")
	}
}

func TestToBackupJobResponse(t *testing.T) {
	now := time.Now()
	errorMsg := "disk full"

	job := &models.BackupJob{
		ID:                "job123",
		BackupID:          "backup123",
		StorageProviderID: "provider123",
		Status:            enums.BackupJobStatusFailed,
		Size:              2 * 1024 * 1024,
		Error:             &errorMsg,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	resp := dto.ToBackupJobResponse(job)

	if resp.ID != "job123" {
		t.Errorf("ID = %s, want job123", resp.ID)
	}
	if resp.BackupID != "backup123" {
		t.Errorf("BackupID = %s, want backup123", resp.BackupID)
	}
	if resp.Status != "failed" {
		t.Errorf("Status = %s, want failed", resp.Status)
	}
	if resp.Size != 2*1024*1024 {
		t.Errorf("Size = %d, want %d", resp.Size, 2*1024*1024)
	}
	if resp.SizeInMB != 2 {
		t.Errorf("SizeInMB = %d, want 2", resp.SizeInMB)
	}
	if resp.Error != "disk full" {
		t.Errorf("Error = %s, want disk full", resp.Error)
	}
}

func TestToBackupJobResponse_NoError(t *testing.T) {
	job := &models.BackupJob{
		ID:        "job123",
		BackupID:  "backup123",
		Status:    enums.BackupJobStatusFinished,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := dto.ToBackupJobResponse(job)

	if resp.Error != "" {
		t.Errorf("Error = %s, want empty", resp.Error)
	}
}

func TestToStorageProviderResponse(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(time.Hour)

	provider := &models.StorageProvider{
		ID:             1,
		UserID:         "user123",
		TeamID:         "team123",
		Provider:       enums.StorageDriverS3,
		Label:          "My S3",
		Connected:      true,
		TokenExpiresAt: &expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	resp := dto.ToStorageProviderResponse(provider)

	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
	if resp.UserID != "user123" {
		t.Errorf("UserID = %s, want user123", resp.UserID)
	}
	if resp.TeamID != "team123" {
		t.Errorf("TeamID = %s, want team123", resp.TeamID)
	}
	if resp.Provider != "s3" {
		t.Errorf("Provider = %s, want s3", resp.Provider)
	}
	if resp.ProviderLabel != "S3" {
		t.Errorf("ProviderLabel = %s, want S3", resp.ProviderLabel)
	}
	if resp.Label != "My S3" {
		t.Errorf("Label = %s, want My S3", resp.Label)
	}
	if !resp.Connected {
		t.Error("expected Connected to be true")
	}
	if resp.TokenExpiresAt == nil {
		t.Error("expected TokenExpiresAt to be set")
	}
}

func TestToStorageProviderResponse_NoExpiry(t *testing.T) {
	provider := &models.StorageProvider{
		ID:        1,
		Provider:  enums.StorageDriverDropbox,
		Label:     "My Dropbox",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := dto.ToStorageProviderResponse(provider)

	if resp.TokenExpiresAt != nil {
		t.Error("expected TokenExpiresAt to be nil")
	}
	if resp.ProviderLabel != "Dropbox" {
		t.Errorf("ProviderLabel = %s, want Dropbox", resp.ProviderLabel)
	}
}

func TestToStorageProviderListItem(t *testing.T) {
	provider := &models.StorageProvider{
		ID:    42,
		Label: "Production S3",
	}

	item := dto.ToStorageProviderListItem(provider)

	if item.ID != 42 {
		t.Errorf("ID = %d, want 42", item.ID)
	}
	if item.Label != "Production S3" {
		t.Errorf("Label = %s, want Production S3", item.Label)
	}
}

func TestAgentBackupConfig(t *testing.T) {
	config := dto.AgentBackupConfig{
		ID:             "backup123",
		CronExpression: "0 0 * * *",
		Path:           "/var/www",
		Retention:      7,
		WebhookURL:     "https://example.com/backup/backup123/token",
		IncludeFiles:   []string{"/app"},
		ExcludeFiles:   []string{"/cache"},
		Databases:      []string{"db1", "db2"},
		Storage: map[string]interface{}{
			"bucket": "my-bucket",
			"region": "us-east-1",
		},
		StorageDriver: "s3",
	}

	if config.ID != "backup123" {
		t.Errorf("ID = %s, want backup123", config.ID)
	}
	if config.CronExpression != "0 0 * * *" {
		t.Errorf("CronExpression = %s, want 0 0 * * *", config.CronExpression)
	}
	if config.Retention != 7 {
		t.Errorf("Retention = %d, want 7", config.Retention)
	}
	if len(config.IncludeFiles) != 1 {
		t.Errorf("IncludeFiles length = %d, want 1", len(config.IncludeFiles))
	}
	if len(config.ExcludeFiles) != 1 {
		t.Errorf("ExcludeFiles length = %d, want 1", len(config.ExcludeFiles))
	}
	if len(config.Databases) != 2 {
		t.Errorf("Databases length = %d, want 2", len(config.Databases))
	}
	if config.Storage["bucket"] != "my-bucket" {
		t.Errorf("Storage[bucket] = %v, want my-bucket", config.Storage["bucket"])
	}
}

func TestCreateBackupRequest_Fields(t *testing.T) {
	req := dto.CreateBackupRequest{
		CronExpression:        "0 0 * * *",
		Path:                  "/var/www",
		Enabled:               true,
		DatabaseID:            "db123",
		StorageProviderID:     "provider123",
		IncludeFiles:          []string{"/app", "/config"},
		ExcludeFiles:          []string{"/cache", "/logs"},
		Retention:             14,
		NotificationOnFailure: true,
		NotificationOnSuccess: false,
	}

	if req.CronExpression != "0 0 * * *" {
		t.Errorf("CronExpression = %s, want 0 0 * * *", req.CronExpression)
	}
	if req.Retention != 14 {
		t.Errorf("Retention = %d, want 14", req.Retention)
	}
	if len(req.IncludeFiles) != 2 {
		t.Errorf("IncludeFiles length = %d, want 2", len(req.IncludeFiles))
	}
}

func TestUpdateBackupRequest_Fields(t *testing.T) {
	req := dto.UpdateBackupRequest{
		CronExpression:        "0 12 * * *",
		Path:                  "/new/path",
		Enabled:               false,
		DatabaseID:            "db456",
		StorageProviderID:     "provider456",
		IncludeFiles:          []string{"/new"},
		ExcludeFiles:          []string{"/old"},
		Retention:             30,
		NotificationOnFailure: false,
		NotificationOnSuccess: true,
	}

	if req.CronExpression != "0 12 * * *" {
		t.Errorf("CronExpression = %s, want 0 12 * * *", req.CronExpression)
	}
	if req.Enabled {
		t.Error("expected Enabled to be false")
	}
}

func TestCreateBackupJobRequest_Fields(t *testing.T) {
	req := dto.CreateBackupJobRequest{
		Status: enums.BackupJobStatusFinished,
		Size:   1024 * 1024,
		Error:  "",
	}

	if req.Status != enums.BackupJobStatusFinished {
		t.Errorf("Status = %s, want finished", req.Status)
	}
	if req.Size != 1024*1024 {
		t.Errorf("Size = %d, want %d", req.Size, 1024*1024)
	}

	// With error
	req = dto.CreateBackupJobRequest{
		Status: enums.BackupJobStatusFailed,
		Error:  "connection timeout",
	}

	if req.Error != "connection timeout" {
		t.Errorf("Error = %s, want connection timeout", req.Error)
	}
}

func TestCreateStorageProviderRequest_S3(t *testing.T) {
	req := dto.CreateStorageProviderRequest{
		Label:          "Production S3",
		Provider:       "s3",
		Endpoint:       "https://s3.custom.com",
		Key:            "access-key",
		Secret:         "secret-key",
		Region:         "us-east-1",
		Bucket:         "prod-bucket",
		Path:           "/backups",
		ForcePathStyle: true,
	}

	if req.Provider != "s3" {
		t.Errorf("Provider = %s, want s3", req.Provider)
	}
	if req.Key != "access-key" {
		t.Errorf("Key = %s, want access-key", req.Key)
	}
	if !req.ForcePathStyle {
		t.Error("expected ForcePathStyle to be true")
	}
}

func TestCreateStorageProviderRequest_Dropbox(t *testing.T) {
	req := dto.CreateStorageProviderRequest{
		Label:    "My Dropbox",
		Provider: "dropbox",
		Token:    "dropbox-token",
	}

	if req.Provider != "dropbox" {
		t.Errorf("Provider = %s, want dropbox", req.Provider)
	}
	if req.Token != "dropbox-token" {
		t.Errorf("Token = %s, want dropbox-token", req.Token)
	}
}

func TestUpdateStorageProviderRequest_Fields(t *testing.T) {
	req := dto.UpdateStorageProviderRequest{
		ID:             42,
		Label:          "Updated Provider",
		Provider:       "s3",
		Key:            "new-key",
		Secret:         "new-secret",
		Region:         "eu-west-1",
		Bucket:         "new-bucket",
		Path:           "/new-path",
		ForcePathStyle: false,
	}

	if req.ID != 42 {
		t.Errorf("ID = %d, want 42", req.ID)
	}
	if req.Label != "Updated Provider" {
		t.Errorf("Label = %s, want Updated Provider", req.Label)
	}
	if req.Region != "eu-west-1" {
		t.Errorf("Region = %s, want eu-west-1", req.Region)
	}
}
