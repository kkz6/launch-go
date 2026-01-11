package backup

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/enums"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
)

func setupTestService(t *testing.T) (*services.BackupService, *services.BackupJobService, *services.StorageProviderService, *repositories.BackupRepository, *repositories.BackupJobRepository, *repositories.StorageProviderRepository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&models.Backup{}, &models.BackupJob{}, &models.StorageProvider{}, &models.BackupDatabase{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	logger := zerolog.Nop()

	backupRepo := repositories.NewBackupRepository(db)
	backupJobRepo := repositories.NewBackupJobRepository(db)
	storageProviderRepo := repositories.NewStorageProviderRepository(db)

	backupService := services.NewBackupService(backupRepo, nil, nil, &logger)
	backupJobService := services.NewBackupJobService(backupJobRepo, backupRepo, nil, &logger)
	storageProviderService := services.NewStorageProviderService(storageProviderRepo, nil, &logger)

	return backupService, backupJobService, storageProviderService, backupRepo, backupJobRepo, storageProviderRepo, db
}

func TestService_CreateBackup(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
		IncludeFiles:      []string{"/app"},
		ExcludeFiles:      []string{"/cache"},
		Retention:         7,
	}

	backup, err := backupService.CreateBackup(ctx, "server123", "user123", req)
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
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
		Retention:         0, // Should default to 10
	}

	backup, err := backupService.CreateBackup(ctx, "server123", "user123", req)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	if backup.Retention != 10 {
		t.Errorf("Retention = %d, want 10 (default)", backup.Retention)
	}
}

func TestService_UpdateBackup(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create initial backup
	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	// Update backup
	updateReq := &dto.UpdateBackupRequest{
		CronExpression:    "0 12 * * *",
		Path:              "/new/path",
		Enabled:           false,
		DatabaseID:        "db456",
		StorageProviderID: "provider456",
		Retention:         15,
	}

	updated, err := backupService.UpdateBackup(ctx, backup.ID, updateReq)
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
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	updateReq := &dto.UpdateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/path",
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}

	_, err := backupService.UpdateBackup(ctx, "nonexistent", updateReq)
	if err != repositories.ErrBackupNotFound {
		t.Errorf("UpdateBackup() error = %v, want %v", err, repositories.ErrBackupNotFound)
	}
}

func TestService_DeleteBackup(t *testing.T) {
	backupService, _, _, backupRepo, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	err := backupService.DeleteBackup(ctx, backup.ID, "server123")
	if err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}

	_, err = backupRepo.FindBackupByID(ctx, backup.ID)
	if err != repositories.ErrBackupNotFound {
		t.Error("expected backup to be deleted")
	}
}

func TestService_DeleteBackup_NotFound(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	err := backupService.DeleteBackup(ctx, "nonexistent", "server123")
	if err != repositories.ErrBackupNotFound {
		t.Errorf("DeleteBackup() error = %v, want %v", err, repositories.ErrBackupNotFound)
	}
}

func TestService_GetBackup(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	found, err := backupService.GetBackup(ctx, backup.ID)
	if err != nil {
		t.Fatalf("GetBackup() error = %v", err)
	}

	if found.ID != backup.ID {
		t.Errorf("GetBackup() ID = %s, want %s", found.ID, backup.ID)
	}
}

func TestService_GetBackupByIDAndServer(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	found, err := backupService.GetBackupByIDAndServer(ctx, backup.ID, "server123")
	if err != nil {
		t.Fatalf("GetBackupByIDAndServer() error = %v", err)
	}
	if found.ID != backup.ID {
		t.Errorf("GetBackupByIDAndServer() ID = %s, want %s", found.ID, backup.ID)
	}

	// Wrong server
	_, err = backupService.GetBackupByIDAndServer(ctx, backup.ID, "wrong-server")
	if err != repositories.ErrBackupNotFound {
		t.Errorf("GetBackupByIDAndServer() error = %v, want %v", err, repositories.ErrBackupNotFound)
	}
}

func TestService_ListBackupsByServer(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create backups for different servers
	req1 := &dto.CreateBackupRequest{CronExpression: "0 0 * * *", Path: "/path1", DatabaseID: "db1", StorageProviderID: "p1"}
	req2 := &dto.CreateBackupRequest{CronExpression: "0 0 * * *", Path: "/path2", DatabaseID: "db2", StorageProviderID: "p2"}
	req3 := &dto.CreateBackupRequest{CronExpression: "0 0 * * *", Path: "/path3", DatabaseID: "db3", StorageProviderID: "p3"}

	backupService.CreateBackup(ctx, "server1", "user1", req1)
	backupService.CreateBackup(ctx, "server1", "user1", req2)
	backupService.CreateBackup(ctx, "server2", "user1", req3)

	backups, err := backupService.ListBackupsByServer(ctx, "server1")
	if err != nil {
		t.Fatalf("ListBackupsByServer() error = %v", err)
	}

	if len(backups) != 2 {
		t.Errorf("ListBackupsByServer() returned %d backups, want 2", len(backups))
	}
}

func TestService_RunBackup(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	err := backupService.RunBackup(ctx, backup.ID, "server123")
	if err != nil {
		t.Fatalf("RunBackup() error = %v", err)
	}
}

func TestService_RunBackup_NotFound(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	err := backupService.RunBackup(ctx, "nonexistent", "server123")
	if err != repositories.ErrBackupNotFound {
		t.Errorf("RunBackup() error = %v, want %v", err, repositories.ErrBackupNotFound)
	}
}

func TestService_CreateBackupJob(t *testing.T) {
	backupService, backupJobService, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	jobReq := &dto.CreateBackupJobRequest{
		Status: enums.BackupJobStatusFinished,
		Size:   1024 * 1024,
	}

	job, err := backupJobService.CreateBackupJob(ctx, backup.ID, backup.DispatchToken, jobReq)
	if err != nil {
		t.Fatalf("CreateBackupJob() error = %v", err)
	}

	if job.Status != enums.BackupJobStatusFinished {
		t.Errorf("Status = %s, want finished", job.Status)
	}
	if job.Size != 1024*1024 {
		t.Errorf("Size = %d, want %d", job.Size, 1024*1024)
	}
}

func TestService_CreateBackupJob_InvalidToken(t *testing.T) {
	backupService, backupJobService, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	jobReq := &dto.CreateBackupJobRequest{Status: enums.BackupJobStatusFinished}

	_, err := backupJobService.CreateBackupJob(ctx, backup.ID, "wrong-token", jobReq)
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestService_CreateBackupJob_WithError(t *testing.T) {
	backupService, backupJobService, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	jobReq := &dto.CreateBackupJobRequest{
		Status: enums.BackupJobStatusFailed,
		Error:  "disk full",
	}

	job, err := backupJobService.CreateBackupJob(ctx, backup.ID, backup.DispatchToken, jobReq)
	if err != nil {
		t.Fatalf("CreateBackupJob() error = %v", err)
	}

	if job.Error == nil || *job.Error != "disk full" {
		t.Error("expected error message to be set")
	}
}

func TestService_GetBackupJob(t *testing.T) {
	backupService, backupJobService, _, _, backupJobRepo, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	job := &models.BackupJob{
		BackupID:          backup.ID,
		StorageProviderID: "provider123",
		Status:            enums.BackupJobStatusFinished,
	}
	backupJobRepo.CreateBackupJob(ctx, job)

	found, err := backupJobService.GetBackupJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetBackupJob() error = %v", err)
	}

	if found.ID != job.ID {
		t.Errorf("GetBackupJob() ID = %s, want %s", found.ID, job.ID)
	}
}

func TestService_ListBackupJobs(t *testing.T) {
	backupService, backupJobService, _, _, backupJobRepo, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	job1 := &models.BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: enums.BackupJobStatusFinished}
	job2 := &models.BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: enums.BackupJobStatusFailed}
	backupJobRepo.CreateBackupJob(ctx, job1)
	backupJobRepo.CreateBackupJob(ctx, job2)

	jobs, err := backupJobService.ListBackupJobs(ctx, backup.ID)
	if err != nil {
		t.Fatalf("ListBackupJobs() error = %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("ListBackupJobs() returned %d jobs, want 2", len(jobs))
	}
}

func TestService_ConnectStorageProvider(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &dto.CreateStorageProviderRequest{
		Label:    "My S3",
		Provider: "s3",
		Key:      "access-key",
		Secret:   "secret-key",
		Region:   "us-east-1",
		Bucket:   "my-bucket",
	}

	provider, err := storageProviderService.ConnectStorageProvider(ctx, "user123", "team123", req)
	if err != nil {
		t.Fatalf("ConnectStorageProvider() error = %v", err)
	}

	if provider.Label != "My S3" {
		t.Errorf("Label = %s, want My S3", provider.Label)
	}
	if provider.Provider != enums.StorageDriverS3 {
		t.Errorf("Provider = %s, want s3", provider.Provider)
	}
	if !provider.Connected {
		t.Error("expected Connected to be true")
	}
}

func TestService_ConnectStorageProvider_InvalidDriver(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &dto.CreateStorageProviderRequest{
		Label:    "Invalid",
		Provider: "invalid",
	}

	_, err := storageProviderService.ConnectStorageProvider(ctx, "user123", "team123", req)
	if err != services.ErrInvalidStorageDriver {
		t.Errorf("ConnectStorageProvider() error = %v, want %v", err, services.ErrInvalidStorageDriver)
	}
}

func TestService_UpdateStorageProvider(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create provider first
	createReq := &dto.CreateStorageProviderRequest{
		Label:    "Original",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := storageProviderService.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	// Update it
	updateReq := &dto.UpdateStorageProviderRequest{
		ID:       provider.ID,
		Label:    "Updated",
		Provider: "s3",
		Key:      "new-key",
		Secret:   "new-secret",
		Region:   "eu-west-1",
		Bucket:   "new-bucket",
	}

	updated, err := storageProviderService.UpdateStorageProvider(ctx, provider.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateStorageProvider() error = %v", err)
	}

	if updated.Label != "Updated" {
		t.Errorf("Label = %s, want Updated", updated.Label)
	}
}

func TestService_UpdateStorageProvider_NotFound(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	updateReq := &dto.UpdateStorageProviderRequest{
		ID:       999999,
		Label:    "Test",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}

	_, err := storageProviderService.UpdateStorageProvider(ctx, 999999, updateReq)
	if err != repositories.ErrStorageProviderNotFound {
		t.Errorf("UpdateStorageProvider() error = %v, want %v", err, repositories.ErrStorageProviderNotFound)
	}
}

func TestService_DeleteStorageProvider(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateStorageProviderRequest{
		Label:    "To Delete",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := storageProviderService.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	err := storageProviderService.DeleteStorageProvider(ctx, provider.ID)
	if err != nil {
		t.Fatalf("DeleteStorageProvider() error = %v", err)
	}

	_, err = storageProviderService.GetStorageProvider(ctx, provider.ID)
	if err != repositories.ErrStorageProviderNotFound {
		t.Error("expected provider to be deleted")
	}
}

func TestService_DeleteStorageProvider_HasBackups(t *testing.T) {
	_, _, storageProviderService, _, _, _, db := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateStorageProviderRequest{
		Label:    "With Backups",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := storageProviderService.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	// Create a backup using this provider
	backup := &models.Backup{
		ServerID:          "server1",
		UserID:            "user1",
		StorageProviderID: "1", // Use the provider ID as string
		CronExpression:    "* * * * *",
		Path:              "/",
	}
	db.Create(backup)

	err := storageProviderService.DeleteStorageProvider(ctx, 1)
	if err != services.ErrStorageProviderHasBackups {
		t.Errorf("DeleteStorageProvider() error = %v, want %v", err, services.ErrStorageProviderHasBackups)
	}

	// Clean up
	db.Delete(backup)
	storageProviderService.DeleteStorageProvider(ctx, provider.ID)
}

func TestService_GetStorageProvider(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	createReq := &dto.CreateStorageProviderRequest{
		Label:    "Test Provider",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := storageProviderService.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	found, err := storageProviderService.GetStorageProvider(ctx, provider.ID)
	if err != nil {
		t.Fatalf("GetStorageProvider() error = %v", err)
	}

	if found.ID != provider.ID {
		t.Errorf("GetStorageProvider() ID = %d, want %d", found.ID, provider.ID)
	}
}

func TestService_ListStorageProvidersByTeam(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	// Use only S3 providers since Dropbox actually tries to connect
	req1 := &dto.CreateStorageProviderRequest{Label: "P1", Provider: "s3", Key: "k1", Secret: "s1", Region: "r1", Bucket: "b1"}
	req2 := &dto.CreateStorageProviderRequest{Label: "P2", Provider: "s3", Key: "k2", Secret: "s2", Region: "r2", Bucket: "b2"}
	req3 := &dto.CreateStorageProviderRequest{Label: "P3", Provider: "s3", Key: "k3", Secret: "s3", Region: "r3", Bucket: "b3"}

	storageProviderService.ConnectStorageProvider(ctx, "user1", "team1", req1)
	storageProviderService.ConnectStorageProvider(ctx, "user1", "team1", req2)
	storageProviderService.ConnectStorageProvider(ctx, "user1", "team2", req3)

	providers, err := storageProviderService.ListStorageProvidersByTeam(ctx, "team1")
	if err != nil {
		t.Fatalf("ListStorageProvidersByTeam() error = %v", err)
	}

	if len(providers) != 2 {
		t.Errorf("ListStorageProvidersByTeam() returned %d providers, want 2", len(providers))
	}
}

func TestService_MarkBackupInstalled(t *testing.T) {
	backupService, _, _, backupRepo, _, _, _ := setupTestService(t)
	ctx := context.Background()

	backup := &models.Backup{
		ServerID:          "server1",
		UserID:            "user1",
		StorageProviderID: "p1",
		CronExpression:    "* * * * *",
		Path:              "/",
	}
	backupRepo.CreateBackup(ctx, backup)

	err := backupService.MarkBackupInstalled(ctx, backup.ID)
	if err != nil {
		t.Fatalf("MarkBackupInstalled() error = %v", err)
	}
}

func TestService_MarkBackupInstallationFailed(t *testing.T) {
	backupService, _, _, backupRepo, _, _, _ := setupTestService(t)
	ctx := context.Background()

	backup := &models.Backup{
		ServerID:          "server1",
		UserID:            "user1",
		StorageProviderID: "p1",
		CronExpression:    "* * * * *",
		Path:              "/",
	}
	backupRepo.CreateBackup(ctx, backup)

	err := backupService.MarkBackupInstallationFailed(ctx, backup.ID)
	if err != nil {
		t.Fatalf("MarkBackupInstallationFailed() error = %v", err)
	}
}

func TestService_GetStorageProviderConfig(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a provider first
	createReq := &dto.CreateStorageProviderRequest{
		Label:    "Config Test",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := storageProviderService.ConnectStorageProvider(ctx, "user1", "team1", createReq)

	config, err := storageProviderService.GetStorageProviderConfig(ctx, provider.ID)
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
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := storageProviderService.GetStorageProviderConfig(ctx, 999999)
	if err != repositories.ErrStorageProviderNotFound {
		t.Errorf("GetStorageProviderConfig() error = %v, want %v", err, repositories.ErrStorageProviderNotFound)
	}
}

func TestService_UpdateStorageProvider_InvalidDriver(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create provider first
	createReq := &dto.CreateStorageProviderRequest{
		Label:    "Original",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	}
	provider, _ := storageProviderService.ConnectStorageProvider(ctx, "user123", "team123", createReq)

	// Try to update with invalid driver
	updateReq := &dto.UpdateStorageProviderRequest{
		ID:       provider.ID,
		Label:    "Updated",
		Provider: "invalid_driver",
	}

	_, err := storageProviderService.UpdateStorageProvider(ctx, provider.ID, updateReq)
	if err != services.ErrInvalidStorageDriver {
		t.Errorf("UpdateStorageProvider() error = %v, want %v", err, services.ErrInvalidStorageDriver)
	}
}

func TestService_CreateBackupJob_BackupNotFound(t *testing.T) {
	_, backupJobService, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	jobReq := &dto.CreateBackupJobRequest{
		Status: enums.BackupJobStatusFinished,
		Size:   1024,
	}

	_, err := backupJobService.CreateBackupJob(ctx, "nonexistent-backup", "some-token", jobReq)
	if err != repositories.ErrBackupNotFound {
		t.Errorf("CreateBackupJob() error = %v, want %v", err, repositories.ErrBackupNotFound)
	}
}

func TestService_DeleteStorageProvider_NotFound(t *testing.T) {
	_, _, storageProviderService, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	err := storageProviderService.DeleteStorageProvider(ctx, 999999)
	if err != repositories.ErrStorageProviderNotFound {
		t.Errorf("DeleteStorageProvider() error = %v, want %v", err, repositories.ErrStorageProviderNotFound)
	}
}

func TestService_UpdateBackup_ZeroRetention(t *testing.T) {
	backupService, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create initial backup with retention
	createReq := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
		Retention:         15,
	}
	backup, _ := backupService.CreateBackup(ctx, "server123", "user123", createReq)

	// Update with retention 0 - should keep existing retention
	updateReq := &dto.UpdateBackupRequest{
		CronExpression:    "0 12 * * *",
		Path:              "/new/path",
		Enabled:           false,
		DatabaseID:        "db456",
		StorageProviderID: "provider456",
		Retention:         0, // Zero should not update
	}

	updated, err := backupService.UpdateBackup(ctx, backup.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateBackup() error = %v", err)
	}

	// Retention should remain 15 since update was 0
	if updated.Retention != 15 {
		t.Errorf("Retention = %d, want 15 (unchanged)", updated.Retention)
	}
}
