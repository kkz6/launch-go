package backup

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestService(t *testing.T) (*Service, *Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&Backup{}, &BackupJob{}, &StorageProvider{}, &BackupDatabase{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	logger := zerolog.Nop()
	repo := NewRepository(db)
	service := NewService(repo, nil, nil, &logger)

	return service, repo, db
}

func TestService_CreateBackup(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
		IncludeFiles:      []string{"/app"},
		ExcludeFiles:      []string{"/cache"},
		Retention:         7,
	}

	backup, err := service.CreateBackup(ctx, "server123", "user123", req)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	if backup.ID == "" {
		t.Error("expected backup ID to be generated")
	}
	if backup.ServerID != "server123" {
		t.Errorf("ServerID = %s, want server123", backup.ServerID)
	}
	if backup.UserID != "user123" {
		t.Errorf("UserID = %s, want user123", backup.UserID)
	}
	if backup.CronExpression != "0 0 * * *" {
		t.Errorf("CronExpression = %s, want 0 0 * * *", backup.CronExpression)
	}
	if backup.Retention != 7 {
		t.Errorf("Retention = %d, want 7", backup.Retention)
	}
}

func TestService_CreateBackup_DefaultRetention(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
		Retention:         0, // Should default to 10
	}

	backup, err := service.CreateBackup(ctx, "server123", "user123", req)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	if backup.Retention != 10 {
		t.Errorf("Retention = %d, want 10 (default)", backup.Retention)
	}
}

func TestService_UpdateBackup(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create initial backup
	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	// Update backup
	updateReq := &UpdateBackupRequest{
		CronExpression:    "0 12 * * *",
		Path:              "/new/path",
		Enabled:           false,
		DatabaseID:        "db456",
		StorageProviderID: "provider456",
		Retention:         15,
	}

	updated, err := service.UpdateBackup(ctx, backup.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateBackup() error = %v", err)
	}

	if updated.CronExpression != "0 12 * * *" {
		t.Errorf("CronExpression = %s, want 0 12 * * *", updated.CronExpression)
	}
	if updated.Path != "/new/path" {
		t.Errorf("Path = %s, want /new/path", updated.Path)
	}
	if updated.Enabled != false {
		t.Error("expected Enabled to be false")
	}
	if updated.Retention != 15 {
		t.Errorf("Retention = %d, want 15", updated.Retention)
	}
}

func TestService_UpdateBackup_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	updateReq := &UpdateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/path",
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}

	_, err := service.UpdateBackup(ctx, "nonexistent", updateReq)
	if err != ErrBackupNotFound {
		t.Errorf("UpdateBackup() error = %v, want %v", err, ErrBackupNotFound)
	}
}

func TestService_DeleteBackup(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	err := service.DeleteBackup(ctx, backup.ID, "server123")
	if err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}

	_, err = repo.FindBackupByID(ctx, backup.ID)
	if err != ErrBackupNotFound {
		t.Error("expected backup to be deleted")
	}
}

func TestService_DeleteBackup_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	err := service.DeleteBackup(ctx, "nonexistent", "server123")
	if err != ErrBackupNotFound {
		t.Errorf("DeleteBackup() error = %v, want %v", err, ErrBackupNotFound)
	}
}

func TestService_GetBackup(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	found, err := service.GetBackup(ctx, backup.ID)
	if err != nil {
		t.Fatalf("GetBackup() error = %v", err)
	}

	if found.ID != backup.ID {
		t.Errorf("GetBackup() ID = %s, want %s", found.ID, backup.ID)
	}
}

func TestService_GetBackupByIDAndServer(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	found, err := service.GetBackupByIDAndServer(ctx, backup.ID, "server123")
	if err != nil {
		t.Fatalf("GetBackupByIDAndServer() error = %v", err)
	}
	if found.ID != backup.ID {
		t.Errorf("GetBackupByIDAndServer() ID = %s, want %s", found.ID, backup.ID)
	}

	// Wrong server
	_, err = service.GetBackupByIDAndServer(ctx, backup.ID, "wrong-server")
	if err != ErrBackupNotFound {
		t.Errorf("GetBackupByIDAndServer() error = %v, want %v", err, ErrBackupNotFound)
	}
}

func TestService_ListBackupsByServer(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create backups for different servers
	req1 := &CreateBackupRequest{CronExpression: "0 0 * * *", Path: "/path1", DatabaseID: "db1", StorageProviderID: "p1"}
	req2 := &CreateBackupRequest{CronExpression: "0 0 * * *", Path: "/path2", DatabaseID: "db2", StorageProviderID: "p2"}
	req3 := &CreateBackupRequest{CronExpression: "0 0 * * *", Path: "/path3", DatabaseID: "db3", StorageProviderID: "p3"}

	service.CreateBackup(ctx, "server1", "user1", req1)
	service.CreateBackup(ctx, "server1", "user1", req2)
	service.CreateBackup(ctx, "server2", "user1", req3)

	backups, err := service.ListBackupsByServer(ctx, "server1")
	if err != nil {
		t.Fatalf("ListBackupsByServer() error = %v", err)
	}

	if len(backups) != 2 {
		t.Errorf("ListBackupsByServer() returned %d backups, want 2", len(backups))
	}
}

func TestService_RunBackup(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	err := service.RunBackup(ctx, backup.ID, "server123")
	if err != nil {
		t.Fatalf("RunBackup() error = %v", err)
	}
}

func TestService_RunBackup_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	err := service.RunBackup(ctx, "nonexistent", "server123")
	if err != ErrBackupNotFound {
		t.Errorf("RunBackup() error = %v, want %v", err, ErrBackupNotFound)
	}
}

func TestService_CreateBackupJob(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	jobReq := &CreateBackupJobRequest{
		Status: BackupJobStatusFinished,
		Size:   1024 * 1024,
	}

	job, err := service.CreateBackupJob(ctx, backup.ID, backup.DispatchToken, jobReq)
	if err != nil {
		t.Fatalf("CreateBackupJob() error = %v", err)
	}

	if job.Status != BackupJobStatusFinished {
		t.Errorf("Status = %s, want finished", job.Status)
	}
	if job.Size != 1024*1024 {
		t.Errorf("Size = %d, want %d", job.Size, 1024*1024)
	}
}

func TestService_CreateBackupJob_InvalidToken(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	jobReq := &CreateBackupJobRequest{Status: BackupJobStatusFinished}

	_, err := service.CreateBackupJob(ctx, backup.ID, "wrong-token", jobReq)
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestService_CreateBackupJob_WithError(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	jobReq := &CreateBackupJobRequest{
		Status: BackupJobStatusFailed,
		Error:  "disk full",
	}

	job, err := service.CreateBackupJob(ctx, backup.ID, backup.DispatchToken, jobReq)
	if err != nil {
		t.Fatalf("CreateBackupJob() error = %v", err)
	}

	if job.Error == nil || *job.Error != "disk full" {
		t.Error("expected error message to be set")
	}
}

func TestService_GetBackupJob(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	job := &BackupJob{
		BackupID:          backup.ID,
		StorageProviderID: "provider123",
		Status:            BackupJobStatusFinished,
	}
	repo.CreateBackupJob(ctx, job)

	found, err := service.GetBackupJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetBackupJob() error = %v", err)
	}

	if found.ID != job.ID {
		t.Errorf("GetBackupJob() ID = %s, want %s", found.ID, job.ID)
	}
}

func TestService_ListBackupJobs(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	job1 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFinished}
	job2 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFailed}
	repo.CreateBackupJob(ctx, job1)
	repo.CreateBackupJob(ctx, job2)

	jobs, err := service.ListBackupJobs(ctx, backup.ID)
	if err != nil {
		t.Fatalf("ListBackupJobs() error = %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("ListBackupJobs() returned %d jobs, want 2", len(jobs))
	}
}

func TestService_ConnectStorageProvider(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &CreateStorageProviderRequest{
		Label:    "My S3",
		Provider: "s3",
		Key:      "access-key",
		Secret:   "secret-key",
		Region:   "us-east-1",
		Bucket:   "my-bucket",
	}

	provider, err := service.ConnectStorageProvider(ctx, "user123", "team123", req)
	if err != nil {
		t.Fatalf("ConnectStorageProvider() error = %v", err)
	}

	if provider.Label != "My S3" {
		t.Errorf("Label = %s, want My S3", provider.Label)
	}
	if provider.Provider != StorageDriverS3 {
		t.Errorf("Provider = %s, want s3", provider.Provider)
	}
	if !provider.Connected {
		t.Error("expected Connected to be true")
	}
}

func TestService_ConnectStorageProvider_InvalidDriver(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &CreateStorageProviderRequest{
		Label:    "Invalid",
		Provider: "invalid",
	}

	_, err := service.ConnectStorageProvider(ctx, "user123", "team123", req)
	if err != ErrInvalidStorageDriver {
		t.Errorf("ConnectStorageProvider() error = %v, want %v", err, ErrInvalidStorageDriver)
	}
}

func TestService_UpdateStorageProvider(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create provider first
	createReq := &CreateStorageProviderRequest{
		Label:    "Original",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := service.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	// Update it
	updateReq := &UpdateStorageProviderRequest{
		ID:       provider.ID,
		Label:    "Updated",
		Provider: "s3",
		Key:      "new-key",
		Secret:   "new-secret",
		Region:   "eu-west-1",
		Bucket:   "new-bucket",
	}

	updated, err := service.UpdateStorageProvider(ctx, provider.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateStorageProvider() error = %v", err)
	}

	if updated.Label != "Updated" {
		t.Errorf("Label = %s, want Updated", updated.Label)
	}
}

func TestService_UpdateStorageProvider_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	updateReq := &UpdateStorageProviderRequest{
		ID:       999999,
		Label:    "Test",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}

	_, err := service.UpdateStorageProvider(ctx, 999999, updateReq)
	if err != ErrStorageProviderNotFound {
		t.Errorf("UpdateStorageProvider() error = %v, want %v", err, ErrStorageProviderNotFound)
	}
}

func TestService_DeleteStorageProvider(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateStorageProviderRequest{
		Label:    "To Delete",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := service.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	err := service.DeleteStorageProvider(ctx, provider.ID)
	if err != nil {
		t.Fatalf("DeleteStorageProvider() error = %v", err)
	}

	_, err = service.GetStorageProvider(ctx, provider.ID)
	if err != ErrStorageProviderNotFound {
		t.Error("expected provider to be deleted")
	}
}

func TestService_DeleteStorageProvider_HasBackups(t *testing.T) {
	service, _, db := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateStorageProviderRequest{
		Label:    "With Backups",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := service.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	// Create a backup using this provider
	backup := &Backup{
		ServerID:          "server1",
		UserID:            "user1",
		StorageProviderID: "1", // Use the provider ID as string
		CronExpression:    "* * * * *",
		Path:              "/",
	}
	db.Create(backup)

	err := service.DeleteStorageProvider(ctx, 1)
	if err != ErrStorageProviderHasBackups {
		t.Errorf("DeleteStorageProvider() error = %v, want %v", err, ErrStorageProviderHasBackups)
	}

	// Clean up
	db.Delete(backup)
	service.DeleteStorageProvider(ctx, provider.ID)
}

func TestService_GetStorageProvider(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &CreateStorageProviderRequest{
		Label:    "Test Provider",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := service.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	found, err := service.GetStorageProvider(ctx, provider.ID)
	if err != nil {
		t.Fatalf("GetStorageProvider() error = %v", err)
	}

	if found.ID != provider.ID {
		t.Errorf("GetStorageProvider() ID = %d, want %d", found.ID, provider.ID)
	}
}

func TestService_ListStorageProvidersByTeam(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Use only S3 providers since Dropbox actually tries to connect
	req1 := &CreateStorageProviderRequest{Label: "P1", Provider: "s3", Key: "k1", Secret: "s1", Region: "r1", Bucket: "b1"}
	req2 := &CreateStorageProviderRequest{Label: "P2", Provider: "s3", Key: "k2", Secret: "s2", Region: "r2", Bucket: "b2"}
	req3 := &CreateStorageProviderRequest{Label: "P3", Provider: "s3", Key: "k3", Secret: "s3", Region: "r3", Bucket: "b3"}

	service.ConnectStorageProvider(ctx, "user1", "team1", req1)
	service.ConnectStorageProvider(ctx, "user1", "team1", req2)
	service.ConnectStorageProvider(ctx, "user1", "team2", req3)

	providers, err := service.ListStorageProvidersByTeam(ctx, "team1")
	if err != nil {
		t.Fatalf("ListStorageProvidersByTeam() error = %v", err)
	}

	if len(providers) != 2 {
		t.Errorf("ListStorageProvidersByTeam() returned %d providers, want 2", len(providers))
	}
}

func TestService_buildCredentials(t *testing.T) {
	service, _, _ := setupTestService(t)

	s3Req := &CreateStorageProviderRequest{
		Provider:       "s3",
		Endpoint:       "https://s3.example.com",
		Key:            "key",
		Secret:         "secret",
		Region:         "us-east-1",
		Bucket:         "bucket",
		Path:           "/path",
		ForcePathStyle: true,
	}

	creds := service.buildCredentials(s3Req)

	if creds["endpoint"] != "https://s3.example.com" {
		t.Errorf("endpoint = %v, want https://s3.example.com", creds["endpoint"])
	}
	if creds["key"] != "key" {
		t.Errorf("key = %v, want key", creds["key"])
	}

	dropboxReq := &CreateStorageProviderRequest{
		Provider: "dropbox",
		Token:    "dropbox-token",
	}

	creds = service.buildCredentials(dropboxReq)

	if creds["token"] != "dropbox-token" {
		t.Errorf("token = %v, want dropbox-token", creds["token"])
	}
}

func TestService_MarkBackupInstalled(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server1",
		UserID:            "user1",
		StorageProviderID: "p1",
		CronExpression:    "* * * * *",
		Path:              "/",
	}
	repo.CreateBackup(ctx, backup)

	err := service.MarkBackupInstalled(ctx, backup.ID)
	if err != nil {
		t.Fatalf("MarkBackupInstalled() error = %v", err)
	}
}

func TestService_MarkBackupInstallationFailed(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server1",
		UserID:            "user1",
		StorageProviderID: "p1",
		CronExpression:    "* * * * *",
		Path:              "/",
	}
	repo.CreateBackup(ctx, backup)

	err := service.MarkBackupInstallationFailed(ctx, backup.ID)
	if err != nil {
		t.Fatalf("MarkBackupInstallationFailed() error = %v", err)
	}
}

func TestService_GetStorageProviderConfig(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a provider first
	createReq := &CreateStorageProviderRequest{
		Label:    "Config Test",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := service.ConnectStorageProvider(ctx, "user1", "team1", createReq)

	config, err := service.GetStorageProviderConfig(ctx, provider.ID)
	if err != nil {
		t.Fatalf("GetStorageProviderConfig() error = %v", err)
	}

	if config == nil {
		t.Error("expected config to be returned")
	}
	if config["region"] != "us-east-1" {
		t.Errorf("config[region] = %v, want us-east-1", config["region"])
	}
}

func TestService_GetStorageProviderConfig_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.GetStorageProviderConfig(ctx, 999999)
	if err != ErrStorageProviderNotFound {
		t.Errorf("GetStorageProviderConfig() error = %v, want %v", err, ErrStorageProviderNotFound)
	}
}

func TestService_GetAgentBackupConfig(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a backup
	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
		IncludeFiles:      []string{"/app"},
		ExcludeFiles:      []string{"/cache"},
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	config, err := service.GetAgentBackupConfig(ctx, backup.ID, "https://example.com")
	if err != nil {
		t.Fatalf("GetAgentBackupConfig() error = %v", err)
	}

	if config.ID != backup.ID {
		t.Errorf("config.ID = %s, want %s", config.ID, backup.ID)
	}
	if config.CronExpression != "0 0 * * *" {
		t.Errorf("config.CronExpression = %s, want 0 0 * * *", config.CronExpression)
	}
	if len(config.IncludeFiles) != 1 {
		t.Errorf("config.IncludeFiles length = %d, want 1", len(config.IncludeFiles))
	}
}

func TestService_GetAgentBackupConfig_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.GetAgentBackupConfig(ctx, "nonexistent", "https://example.com")
	if err != ErrBackupNotFound {
		t.Errorf("GetAgentBackupConfig() error = %v, want %v", err, ErrBackupNotFound)
	}
}

func TestService_UpdateStorageProvider_InvalidDriver(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create provider first
	createReq := &CreateStorageProviderRequest{
		Label:    "Original",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := service.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	// Try to update with invalid driver
	updateReq := &UpdateStorageProviderRequest{
		ID:       provider.ID,
		Label:    "Updated",
		Provider: "invalid_driver",
	}

	_, err := service.UpdateStorageProvider(ctx, provider.ID, updateReq)
	if err != ErrInvalidStorageDriver {
		t.Errorf("UpdateStorageProvider() error = %v, want %v", err, ErrInvalidStorageDriver)
	}
}

func TestService_buildCredentialsFromUpdate(t *testing.T) {
	service, _, _ := setupTestService(t)

	// Test S3 credentials
	s3Req := &UpdateStorageProviderRequest{
		ID:             1,
		Provider:       "s3",
		Endpoint:       "https://s3.example.com",
		Key:            "update-key",
		Secret:         "update-secret",
		Region:         "eu-west-1",
		Bucket:         "update-bucket",
		Path:           "/update-path",
		ForcePathStyle: true,
	}

	creds := service.buildCredentialsFromUpdate(s3Req)

	if creds["endpoint"] != "https://s3.example.com" {
		t.Errorf("endpoint = %v, want https://s3.example.com", creds["endpoint"])
	}
	if creds["key"] != "update-key" {
		t.Errorf("key = %v, want update-key", creds["key"])
	}
	if creds["region"] != "eu-west-1" {
		t.Errorf("region = %v, want eu-west-1", creds["region"])
	}

	// Test Dropbox credentials
	dropboxReq := &UpdateStorageProviderRequest{
		ID:       1,
		Provider: "dropbox",
		Token:    "updated-dropbox-token",
	}

	creds = service.buildCredentialsFromUpdate(dropboxReq)

	if creds["token"] != "updated-dropbox-token" {
		t.Errorf("token = %v, want updated-dropbox-token", creds["token"])
	}
}

func TestService_CreateBackupJob_BackupNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	jobReq := &CreateBackupJobRequest{
		Status: BackupJobStatusFinished,
		Size:   1024,
	}

	_, err := service.CreateBackupJob(ctx, "nonexistent-backup", "some-token", jobReq)
	if err != ErrBackupNotFound {
		t.Errorf("CreateBackupJob() error = %v, want %v", err, ErrBackupNotFound)
	}
}

func TestService_DeleteStorageProvider_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	err := service.DeleteStorageProvider(ctx, 999999)
	if err != ErrStorageProviderNotFound {
		t.Errorf("DeleteStorageProvider() error = %v, want %v", err, ErrStorageProviderNotFound)
	}
}

func TestService_UpdateBackup_ZeroRetention(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create initial backup with retention
	createReq := &CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
		Retention:         15,
	}
	backup, _ := service.CreateBackup(ctx, "server123", "user123", createReq)

	// Update with retention 0 - should keep existing retention
	updateReq := &UpdateBackupRequest{
		CronExpression:    "0 12 * * *",
		Path:              "/new/path",
		Enabled:           false,
		DatabaseID:        "db456",
		StorageProviderID: "provider456",
		Retention:         0, // Zero should not update
	}

	updated, err := service.UpdateBackup(ctx, backup.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateBackup() error = %v", err)
	}

	// Retention should remain 15 since update was 0
	if updated.Retention != 15 {
		t.Errorf("Retention = %d, want 15 (unchanged)", updated.Retention)
	}
}
