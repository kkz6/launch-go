package backup

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRepository(t *testing.T) (*Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&Backup{}, &BackupJob{}, &StorageProvider{}, &BackupDatabase{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	repo := NewRepository(db)
	return repo, db
}

func TestRepository_CreateBackup(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}

	err := repo.CreateBackup(ctx, backup)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	if backup.ID == "" {
		t.Error("expected backup ID to be generated")
	}
}

func TestRepository_CreateBackupWithDatabases(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}

	databaseIDs := []string{"db1", "db2", "db3"}

	err := repo.CreateBackupWithDatabases(ctx, backup, databaseIDs)
	if err != nil {
		t.Fatalf("CreateBackupWithDatabases() error = %v", err)
	}

	// Verify databases were associated
	ids, err := repo.GetBackupDatabaseIDs(ctx, backup.ID)
	if err != nil {
		t.Fatalf("GetBackupDatabaseIDs() error = %v", err)
	}

	if len(ids) != len(databaseIDs) {
		t.Errorf("expected %d databases, got %d", len(databaseIDs), len(ids))
	}
}

func TestRepository_FindBackupByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}
	repo.CreateBackup(ctx, backup)

	found, err := repo.FindBackupByID(ctx, backup.ID)
	if err != nil {
		t.Fatalf("FindBackupByID() error = %v", err)
	}

	if found.ID != backup.ID {
		t.Errorf("FindBackupByID() ID = %s, want %s", found.ID, backup.ID)
	}
}

func TestRepository_FindBackupByID_NotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := repo.FindBackupByID(ctx, "nonexistent")
	if err != ErrBackupNotFound {
		t.Errorf("FindBackupByID() error = %v, want %v", err, ErrBackupNotFound)
	}
}

func TestRepository_FindBackupByIDAndServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}
	repo.CreateBackup(ctx, backup)

	found, err := repo.FindBackupByIDAndServer(ctx, backup.ID, "server123")
	if err != nil {
		t.Fatalf("FindBackupByIDAndServer() error = %v", err)
	}

	if found.ID != backup.ID {
		t.Errorf("FindBackupByIDAndServer() ID = %s, want %s", found.ID, backup.ID)
	}

	// Wrong server
	_, err = repo.FindBackupByIDAndServer(ctx, backup.ID, "wrong-server")
	if err != ErrBackupNotFound {
		t.Errorf("FindBackupByIDAndServer() error = %v, want %v", err, ErrBackupNotFound)
	}
}

func TestRepository_FindBackupsByServerID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	// Create backups for different servers
	backup1 := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	backup2 := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	backup3 := &Backup{ServerID: "server2", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}

	repo.CreateBackup(ctx, backup1)
	repo.CreateBackup(ctx, backup2)
	repo.CreateBackup(ctx, backup3)

	backups, err := repo.FindBackupsByServerID(ctx, "server1")
	if err != nil {
		t.Fatalf("FindBackupsByServerID() error = %v", err)
	}

	if len(backups) != 2 {
		t.Errorf("FindBackupsByServerID() returned %d backups, want 2", len(backups))
	}
}

func TestRepository_UpdateBackup(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
	}
	repo.CreateBackup(ctx, backup)

	backup.Enabled = false
	backup.Path = "/new/path"

	err := repo.UpdateBackup(ctx, backup)
	if err != nil {
		t.Fatalf("UpdateBackup() error = %v", err)
	}

	found, _ := repo.FindBackupByID(ctx, backup.ID)
	if found.Enabled != false {
		t.Error("expected Enabled to be false")
	}
	if found.Path != "/new/path" {
		t.Errorf("Path = %s, want /new/path", found.Path)
	}
}

func TestRepository_UpdateBackupWithDatabases(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}
	repo.CreateBackupWithDatabases(ctx, backup, []string{"db1", "db2"})

	backup.Path = "/updated/path"
	err := repo.UpdateBackupWithDatabases(ctx, backup, []string{"db3", "db4", "db5"})
	if err != nil {
		t.Fatalf("UpdateBackupWithDatabases() error = %v", err)
	}

	ids, _ := repo.GetBackupDatabaseIDs(ctx, backup.ID)
	if len(ids) != 3 {
		t.Errorf("expected 3 databases, got %d", len(ids))
	}
}

func TestRepository_UpdateBackupFields(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Retention:         10,
	}
	repo.CreateBackup(ctx, backup)

	err := repo.UpdateBackupFields(ctx, backup.ID, map[string]interface{}{
		"retention": 20,
		"path":      "/updated",
	})
	if err != nil {
		t.Fatalf("UpdateBackupFields() error = %v", err)
	}

	found, _ := repo.FindBackupByID(ctx, backup.ID)
	if found.Retention != 20 {
		t.Errorf("Retention = %d, want 20", found.Retention)
	}
	if found.Path != "/updated" {
		t.Errorf("Path = %s, want /updated", found.Path)
	}
}

func TestRepository_DeleteBackup(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}
	repo.CreateBackup(ctx, backup)

	err := repo.DeleteBackup(ctx, backup.ID)
	if err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}

	_, err = repo.FindBackupByID(ctx, backup.ID)
	if err != ErrBackupNotFound {
		t.Error("expected backup to be deleted")
	}
}

func TestRepository_GetLatestBackupByServerID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup1 := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/first"}
	backup2 := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/second"}

	repo.CreateBackup(ctx, backup1)
	repo.CreateBackup(ctx, backup2)

	latest, err := repo.GetLatestBackupByServerID(ctx, "server1")
	if err != nil {
		t.Fatalf("GetLatestBackupByServerID() error = %v", err)
	}

	if latest.ID != backup2.ID {
		t.Error("expected the second backup to be returned as latest")
	}
}

func TestRepository_GetLatestBackupByServerID_None(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	latest, err := repo.GetLatestBackupByServerID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetLatestBackupByServerID() error = %v", err)
	}

	if latest != nil {
		t.Error("expected nil for nonexistent server")
	}
}

func TestRepository_BackupJob_CRUD(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	repo.CreateBackup(ctx, backup)

	// Create
	job := &BackupJob{
		BackupID:          backup.ID,
		StorageProviderID: "p1",
		Status:            BackupJobStatusPending,
		Size:              1024,
	}
	err := repo.CreateBackupJob(ctx, job)
	if err != nil {
		t.Fatalf("CreateBackupJob() error = %v", err)
	}

	// Find
	found, err := repo.FindBackupJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("FindBackupJobByID() error = %v", err)
	}
	if found.ID != job.ID {
		t.Errorf("FindBackupJobByID() ID = %s, want %s", found.ID, job.ID)
	}

	// Update
	job.Status = BackupJobStatusFinished
	job.Size = 2048
	err = repo.UpdateBackupJob(ctx, job)
	if err != nil {
		t.Fatalf("UpdateBackupJob() error = %v", err)
	}

	found, _ = repo.FindBackupJobByID(ctx, job.ID)
	if found.Status != BackupJobStatusFinished {
		t.Errorf("Status = %s, want finished", found.Status)
	}

	// Delete
	err = repo.DeleteBackupJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("DeleteBackupJob() error = %v", err)
	}

	_, err = repo.FindBackupJobByID(ctx, job.ID)
	if err != ErrBackupJobNotFound {
		t.Error("expected job to be deleted")
	}
}

func TestRepository_FindBackupJobsByBackupID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	repo.CreateBackup(ctx, backup)

	job1 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFinished}
	job2 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFailed}
	repo.CreateBackupJob(ctx, job1)
	repo.CreateBackupJob(ctx, job2)

	jobs, err := repo.FindBackupJobsByBackupID(ctx, backup.ID)
	if err != nil {
		t.Fatalf("FindBackupJobsByBackupID() error = %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("FindBackupJobsByBackupID() returned %d jobs, want 2", len(jobs))
	}
}

func TestRepository_FindFinishedBackupJobs(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	repo.CreateBackup(ctx, backup)

	job1 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFinished}
	job2 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFailed}
	job3 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFinished}
	repo.CreateBackupJob(ctx, job1)
	repo.CreateBackupJob(ctx, job2)
	repo.CreateBackupJob(ctx, job3)

	jobs, err := repo.FindFinishedBackupJobs(ctx, backup.ID)
	if err != nil {
		t.Fatalf("FindFinishedBackupJobs() error = %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("FindFinishedBackupJobs() returned %d jobs, want 2", len(jobs))
	}
}

func TestRepository_GetBackupJobsTotalSize(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	repo.CreateBackup(ctx, backup)

	job1 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFinished, Size: 1000}
	job2 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFailed, Size: 500}
	job3 := &BackupJob{BackupID: backup.ID, StorageProviderID: "p1", Status: BackupJobStatusFinished, Size: 2000}
	repo.CreateBackupJob(ctx, job1)
	repo.CreateBackupJob(ctx, job2)
	repo.CreateBackupJob(ctx, job3)

	total, err := repo.GetBackupJobsTotalSize(ctx, backup.ID)
	if err != nil {
		t.Fatalf("GetBackupJobsTotalSize() error = %v", err)
	}

	// Only finished jobs should be counted
	expected := int64(3000)
	if total != expected {
		t.Errorf("GetBackupJobsTotalSize() = %d, want %d", total, expected)
	}
}

func TestRepository_StorageProvider_CRUD(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	// Create
	provider := &StorageProvider{
		UserID:   "user1",
		TeamID:   "team1",
		Provider: StorageDriverS3,
		Label:    "My S3",
	}
	err := repo.CreateStorageProvider(ctx, provider)
	if err != nil {
		t.Fatalf("CreateStorageProvider() error = %v", err)
	}

	// Find by ID
	found, err := repo.FindStorageProviderByID(ctx, provider.ID)
	if err != nil {
		t.Fatalf("FindStorageProviderByID() error = %v", err)
	}
	if found.Label != "My S3" {
		t.Errorf("Label = %s, want My S3", found.Label)
	}

	// Update
	provider.Label = "Updated S3"
	err = repo.UpdateStorageProvider(ctx, provider)
	if err != nil {
		t.Fatalf("UpdateStorageProvider() error = %v", err)
	}

	found, _ = repo.FindStorageProviderByID(ctx, provider.ID)
	if found.Label != "Updated S3" {
		t.Errorf("Label = %s, want Updated S3", found.Label)
	}

	// Delete
	err = repo.DeleteStorageProvider(ctx, provider.ID)
	if err != nil {
		t.Fatalf("DeleteStorageProvider() error = %v", err)
	}

	_, err = repo.FindStorageProviderByID(ctx, provider.ID)
	if err != ErrStorageProviderNotFound {
		t.Error("expected provider to be deleted")
	}
}

func TestRepository_FindStorageProvidersByTeamID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	provider1 := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverS3, Label: "S3 1"}
	provider2 := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverDropbox, Label: "Dropbox 1"}
	provider3 := &StorageProvider{UserID: "user1", TeamID: "team2", Provider: StorageDriverS3, Label: "S3 2"}

	repo.CreateStorageProvider(ctx, provider1)
	repo.CreateStorageProvider(ctx, provider2)
	repo.CreateStorageProvider(ctx, provider3)

	providers, err := repo.FindStorageProvidersByTeamID(ctx, "team1")
	if err != nil {
		t.Fatalf("FindStorageProvidersByTeamID() error = %v", err)
	}

	if len(providers) != 2 {
		t.Errorf("FindStorageProvidersByTeamID() returned %d providers, want 2", len(providers))
	}
}

func TestRepository_FindStorageProvidersByDriver(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	provider1 := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverS3, Label: "S3 1"}
	provider2 := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverDropbox, Label: "Dropbox 1"}
	provider3 := &StorageProvider{UserID: "user1", TeamID: "team2", Provider: StorageDriverS3, Label: "S3 2"}

	repo.CreateStorageProvider(ctx, provider1)
	repo.CreateStorageProvider(ctx, provider2)
	repo.CreateStorageProvider(ctx, provider3)

	providers, err := repo.FindStorageProvidersByDriver(ctx, StorageDriverS3)
	if err != nil {
		t.Fatalf("FindStorageProvidersByDriver() error = %v", err)
	}

	if len(providers) != 2 {
		t.Errorf("FindStorageProvidersByDriver() returned %d providers, want 2", len(providers))
	}
}

func TestRepository_FindStorageProvidersByTeamAndDriver(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	provider1 := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverS3, Label: "S3 1"}
	provider2 := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverDropbox, Label: "Dropbox 1"}
	provider3 := &StorageProvider{UserID: "user1", TeamID: "team2", Provider: StorageDriverS3, Label: "S3 2"}

	repo.CreateStorageProvider(ctx, provider1)
	repo.CreateStorageProvider(ctx, provider2)
	repo.CreateStorageProvider(ctx, provider3)

	providers, err := repo.FindStorageProvidersByTeamAndDriver(ctx, "team1", StorageDriverS3)
	if err != nil {
		t.Fatalf("FindStorageProvidersByTeamAndDriver() error = %v", err)
	}

	if len(providers) != 1 {
		t.Errorf("FindStorageProvidersByTeamAndDriver() returned %d providers, want 1", len(providers))
	}
}

func TestRepository_SyncBackupDatabases(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	repo.CreateBackupWithDatabases(ctx, backup, []string{"db1", "db2"})

	err := repo.SyncBackupDatabases(ctx, backup.ID, []string{"db3", "db4"})
	if err != nil {
		t.Fatalf("SyncBackupDatabases() error = %v", err)
	}

	ids, _ := repo.GetBackupDatabaseIDs(ctx, backup.ID)
	if len(ids) != 2 {
		t.Errorf("expected 2 databases, got %d", len(ids))
	}
}

func TestRepository_BackupExists(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	repo.CreateBackup(ctx, backup)

	exists, err := repo.BackupExists(ctx, backup.ID)
	if err != nil {
		t.Fatalf("BackupExists() error = %v", err)
	}
	if !exists {
		t.Error("expected backup to exist")
	}

	exists, err = repo.BackupExists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("BackupExists() error = %v", err)
	}
	if exists {
		t.Error("expected backup to not exist")
	}
}

func TestRepository_StorageProviderExists(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	provider := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverS3, Label: "S3"}
	repo.CreateStorageProvider(ctx, provider)

	exists, err := repo.StorageProviderExists(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StorageProviderExists() error = %v", err)
	}
	if !exists {
		t.Error("expected provider to exist")
	}

	exists, err = repo.StorageProviderExists(ctx, 999999)
	if err != nil {
		t.Fatalf("StorageProviderExists() error = %v", err)
	}
	if exists {
		t.Error("expected provider to not exist")
	}
}

func TestRepository_HasBackupsForStorageProvider(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	provider := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverS3, Label: "S3"}
	repo.CreateStorageProvider(ctx, provider)

	// No backups yet
	hasBackups, err := repo.HasBackupsForStorageProvider(ctx, provider.ID)
	if err != nil {
		t.Fatalf("HasBackupsForStorageProvider() error = %v", err)
	}
	if hasBackups {
		t.Error("expected no backups")
	}

	// Create a backup using this provider - need to use string ID
	backup := &Backup{
		ServerID:          "server1",
		UserID:            "user1",
		StorageProviderID: "1", // First auto-increment ID
		CronExpression:    "* * * * *",
		Path:              "/",
	}
	db.Create(backup)

	hasBackups, err = repo.HasBackupsForStorageProvider(ctx, 1)
	if err != nil {
		t.Fatalf("HasBackupsForStorageProvider() error = %v", err)
	}
	if !hasBackups {
		t.Error("expected to have backups")
	}
}

func TestRepository_FindStorageProviderByIDString(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	provider := &StorageProvider{UserID: "user1", TeamID: "team1", Provider: StorageDriverS3, Label: "S3"}
	repo.CreateStorageProvider(ctx, provider)

	// Note: SQLite uses auto-increment so the string ID would be "1"
	found, err := repo.FindStorageProviderByIDString(ctx, "1")
	if err != nil {
		t.Fatalf("FindStorageProviderByIDString() error = %v", err)
	}
	if found.Label != "S3" {
		t.Errorf("Label = %s, want S3", found.Label)
	}
}

func TestRepository_FindStorageProviderByIDString_NotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := repo.FindStorageProviderByIDString(ctx, "999999")
	if err != ErrStorageProviderNotFound {
		t.Errorf("FindStorageProviderByIDString() error = %v, want %v", err, ErrStorageProviderNotFound)
	}
}

func TestRepository_FindBackupJobByID_NotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := repo.FindBackupJobByID(ctx, "nonexistent")
	if err != ErrBackupJobNotFound {
		t.Errorf("FindBackupJobByID() error = %v, want %v", err, ErrBackupJobNotFound)
	}
}

func TestRepository_GetBackupDatabaseIDs_Empty(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	backup := &Backup{ServerID: "server1", UserID: "user1", StorageProviderID: "p1", CronExpression: "* * * * *", Path: "/"}
	repo.CreateBackup(ctx, backup)

	// Get IDs for backup with no databases
	ids, err := repo.GetBackupDatabaseIDs(ctx, backup.ID)
	if err != nil {
		t.Fatalf("GetBackupDatabaseIDs() error = %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("expected 0 databases, got %d", len(ids))
	}
}
