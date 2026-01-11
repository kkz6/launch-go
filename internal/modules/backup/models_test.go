package backup

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/enums"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(&models.Backup{}, &models.BackupJob{}, &models.StorageProvider{}, &models.BackupDatabase{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func TestBackup_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	backup := &models.Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}

	err := db.Create(backup).Error
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if backup.ID == "" {
		t.Error("expected ID to be generated")
	}

	if backup.DispatchToken == "" {
		t.Error("expected DispatchToken to be generated")
	}

	if backup.Retention != 10 {
		t.Errorf("expected Retention to be 10, got %d", backup.Retention)
	}
}

func TestBackup_BeforeCreate_WithExistingValues(t *testing.T) {
	db := setupTestDB(t)

	backup := &models.Backup{
		ID:                "existing-id",
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		DispatchToken:     "existing-token",
		Retention:         5,
	}

	err := db.Create(backup).Error
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if backup.ID != "existing-id" {
		t.Errorf("expected ID to remain 'existing-id', got %s", backup.ID)
	}

	if backup.DispatchToken != "existing-token" {
		t.Errorf("expected DispatchToken to remain 'existing-token', got %s", backup.DispatchToken)
	}

	if backup.Retention != 5 {
		t.Errorf("expected Retention to remain 5, got %d", backup.Retention)
	}
}

func TestBackup_GetSizeInMB(t *testing.T) {
	backup := &models.Backup{
		Jobs: []models.BackupJob{
			{Size: 1024 * 1024},     // 1 MB
			{Size: 2 * 1024 * 1024}, // 2 MB
			{Size: 512 * 1024},      // 0.5 MB (will round down)
		},
	}

	sizeInMB := backup.GetSizeInMB()
	expected := int64(3) // 1 + 2 + 0 (512KB rounds down to 0)

	if sizeInMB != expected {
		t.Errorf("GetSizeInMB() = %d, want %d", sizeInMB, expected)
	}
}

func TestBackup_GetSizeInMB_Empty(t *testing.T) {
	backup := &models.Backup{}

	sizeInMB := backup.GetSizeInMB()
	if sizeInMB != 0 {
		t.Errorf("GetSizeInMB() = %d, want 0", sizeInMB)
	}
}

func TestBackup_IsInstalled(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		backup models.Backup
		want   bool
	}{
		{
			name: "installed",
			backup: models.Backup{
				InstalledAt:          &now,
				InstallationFailedAt: nil,
			},
			want: true,
		},
		{
			name: "not installed",
			backup: models.Backup{
				InstalledAt:          nil,
				InstallationFailedAt: nil,
			},
			want: false,
		},
		{
			name: "installation failed",
			backup: models.Backup{
				InstalledAt:          &now,
				InstallationFailedAt: &now,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.backup.IsInstalled(); got != tt.want {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackup_IsPendingInstallation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		backup models.Backup
		want   bool
	}{
		{
			name: "pending",
			backup: models.Backup{
				InstalledAt:          nil,
				InstallationFailedAt: nil,
			},
			want: true,
		},
		{
			name: "installed",
			backup: models.Backup{
				InstalledAt:          &now,
				InstallationFailedAt: nil,
			},
			want: false,
		},
		{
			name: "failed",
			backup: models.Backup{
				InstalledAt:          nil,
				InstallationFailedAt: &now,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.backup.IsPendingInstallation(); got != tt.want {
				t.Errorf("IsPendingInstallation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackup_IsInstallationFailed(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		backup models.Backup
		want   bool
	}{
		{
			name: "failed",
			backup: models.Backup{
				InstallationFailedAt: &now,
			},
			want: true,
		},
		{
			name: "not failed",
			backup: models.Backup{
				InstallationFailedAt: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.backup.IsInstallationFailed(); got != tt.want {
				t.Errorf("IsInstallationFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackup_TableName(t *testing.T) {
	backup := models.Backup{}
	if got := backup.TableName(); got != "backups" {
		t.Errorf("TableName() = %s, want backups", got)
	}
}

func TestBackupJob_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a backup
	backup := &models.Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}
	db.Create(backup)

	job := &models.BackupJob{
		BackupID:          backup.ID,
		StorageProviderID: "provider123",
	}

	err := db.Create(job).Error
	if err != nil {
		t.Fatalf("failed to create backup job: %v", err)
	}

	if job.ID == "" {
		t.Error("expected ID to be generated")
	}

	if job.Status != enums.BackupJobStatusPending {
		t.Errorf("expected Status to be pending, got %s", job.Status)
	}
}

func TestBackupJob_GetSizeInMB(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want int64
	}{
		{"1 MB", 1024 * 1024, 1},
		{"10 MB", 10 * 1024 * 1024, 10},
		{"500 KB", 500 * 1024, 0},
		{"0 bytes", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.BackupJob{Size: tt.size}
			if got := job.GetSizeInMB(); got != tt.want {
				t.Errorf("GetSizeInMB() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBackupJob_StatusMethods(t *testing.T) {
	tests := []struct {
		name       string
		status     enums.BackupJobStatus
		isFinished bool
		isFailed   bool
		isRunning  bool
		isPending  bool
	}{
		{"pending", enums.BackupJobStatusPending, false, false, false, true},
		{"running", enums.BackupJobStatusRunning, false, false, true, false},
		{"finished", enums.BackupJobStatusFinished, true, false, false, false},
		{"failed", enums.BackupJobStatusFailed, false, true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.BackupJob{Status: tt.status}

			if got := job.IsFinished(); got != tt.isFinished {
				t.Errorf("IsFinished() = %v, want %v", got, tt.isFinished)
			}
			if got := job.IsFailed(); got != tt.isFailed {
				t.Errorf("IsFailed() = %v, want %v", got, tt.isFailed)
			}
			if got := job.IsRunning(); got != tt.isRunning {
				t.Errorf("IsRunning() = %v, want %v", got, tt.isRunning)
			}
			if got := job.IsPending(); got != tt.isPending {
				t.Errorf("IsPending() = %v, want %v", got, tt.isPending)
			}
		})
	}
}

func TestBackupJob_TableName(t *testing.T) {
	job := models.BackupJob{}
	if got := job.TableName(); got != "backup_jobs" {
		t.Errorf("TableName() = %s, want backup_jobs", got)
	}
}

func TestStorageProvider_TableName(t *testing.T) {
	provider := models.StorageProvider{}
	if got := provider.TableName(); got != "storage_providers" {
		t.Errorf("TableName() = %s, want storage_providers", got)
	}
}

func TestStorageProvider_Credentials(t *testing.T) {
	provider := &models.StorageProvider{}

	creds := map[string]interface{}{
		"key":    "test-key",
		"secret": "test-secret",
		"region": "us-east-1",
	}

	err := provider.SetCredentials(creds)
	if err != nil {
		t.Fatalf("SetCredentials() error = %v", err)
	}

	got, err := provider.GetCredentials()
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}

	if got["key"] != "test-key" {
		t.Errorf("GetCredentials()[key] = %v, want test-key", got["key"])
	}
	if got["secret"] != "test-secret" {
		t.Errorf("GetCredentials()[secret] = %v, want test-secret", got["secret"])
	}
	if got["region"] != "us-east-1" {
		t.Errorf("GetCredentials()[region] = %v, want us-east-1", got["region"])
	}
}

func TestStorageProvider_GetCredentials_Empty(t *testing.T) {
	provider := &models.StorageProvider{}

	got, err := provider.GetCredentials()
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}

	if len(got) != 0 {
		t.Errorf("GetCredentials() returned %d items, want 0", len(got))
	}
}

func TestBackupDatabase_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	backup := &models.Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}
	db.Create(backup)

	backupDB := &models.BackupDatabase{
		BackupID:   backup.ID,
		DatabaseID: "db123",
	}

	err := db.Create(backupDB).Error
	if err != nil {
		t.Fatalf("failed to create backup database: %v", err)
	}

	if backupDB.ID == "" {
		t.Error("expected ID to be generated")
	}
}

func TestBackupDatabase_TableName(t *testing.T) {
	backupDB := models.BackupDatabase{}
	if got := backupDB.TableName(); got != "backup_databases" {
		t.Errorf("TableName() = %s, want backup_databases", got)
	}
}

func TestJSON_MarshalUnmarshal(t *testing.T) {
	original := []string{"file1.txt", "file2.txt", "dir/file3.txt"}

	jsonData, err := models.FromStringSlice(original)
	if err != nil {
		t.Fatalf("FromStringSlice() error = %v", err)
	}

	result, err := jsonData.ToStringSlice()
	if err != nil {
		t.Fatalf("ToStringSlice() error = %v", err)
	}

	if len(result) != len(original) {
		t.Fatalf("ToStringSlice() returned %d items, want %d", len(result), len(original))
	}

	for i, v := range original {
		if result[i] != v {
			t.Errorf("ToStringSlice()[%d] = %s, want %s", i, result[i], v)
		}
	}
}

func TestJSON_Empty(t *testing.T) {
	var j models.JSON

	result, err := j.ToStringSlice()
	if err != nil {
		t.Fatalf("ToStringSlice() error = %v", err)
	}

	if len(result) != 0 {
		t.Errorf("ToStringSlice() returned %d items, want 0", len(result))
	}
}

func TestJSON_NilSlice(t *testing.T) {
	jsonData, err := models.FromStringSlice(nil)
	if err != nil {
		t.Fatalf("FromStringSlice(nil) error = %v", err)
	}

	result, err := jsonData.ToStringSlice()
	if err != nil {
		t.Fatalf("ToStringSlice() error = %v", err)
	}

	if len(result) != 0 {
		t.Errorf("ToStringSlice() returned %d items, want 0", len(result))
	}
}

func TestJSON_Value(t *testing.T) {
	tests := []struct {
		name string
		j    models.JSON
		want string
	}{
		{"empty", models.JSON{}, "[]"},
		{"with data", models.JSON(`["a","b"]`), `["a","b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.j.Value()
			if err != nil {
				t.Fatalf("Value() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJSON_Scan(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  models.JSON
	}{
		{"nil", nil, models.JSON("[]")},
		{"bytes", []byte(`["a","b"]`), models.JSON(`["a","b"]`)},
		{"string", `["c","d"]`, models.JSON(`["c","d"]`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j models.JSON
			err := j.Scan(tt.value)
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}
			if string(j) != string(tt.want) {
				t.Errorf("Scan() = %s, want %s", string(j), string(tt.want))
			}
		})
	}
}

func TestJSON_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		j    models.JSON
		want string
	}{
		{"empty", models.JSON{}, "[]"},
		{"with data", models.JSON(`["a","b"]`), `["a","b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.j.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("MarshalJSON() = %s, want %s", string(got), tt.want)
			}
		})
	}
}

func TestJSON_UnmarshalJSON(t *testing.T) {
	var j models.JSON
	data := []byte(`["x","y","z"]`)

	err := j.UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if string(j) != string(data) {
		t.Errorf("UnmarshalJSON() = %s, want %s", string(j), string(data))
	}
}

func TestEncryptedJSON_NewAndDecrypt(t *testing.T) {
	data := map[string]interface{}{
		"key":    "value",
		"number": float64(42),
	}

	encrypted, err := models.NewEncryptedJSON(data)
	if err != nil {
		t.Fatalf("NewEncryptedJSON() error = %v", err)
	}

	decrypted, err := encrypted.Decrypt()
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if decrypted["key"] != "value" {
		t.Errorf("Decrypt()[key] = %v, want value", decrypted["key"])
	}
	if decrypted["number"] != float64(42) {
		t.Errorf("Decrypt()[number] = %v, want 42", decrypted["number"])
	}
}

func TestEncryptedJSON_Empty(t *testing.T) {
	var e models.EncryptedJSON

	result, err := e.Decrypt()
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Decrypt() returned %d items, want 0", len(result))
	}
}

func TestEncryptedJSON_Value(t *testing.T) {
	tests := []struct {
		name string
		e    models.EncryptedJSON
		want string
	}{
		{"empty", models.EncryptedJSON{}, ""},
		{"with data", models.EncryptedJSON(`{"key":"value"}`), `{"key":"value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.e.Value()
			if err != nil {
				t.Fatalf("Value() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncryptedJSON_Scan(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  models.EncryptedJSON
	}{
		{"nil", nil, models.EncryptedJSON{}},
		{"bytes", []byte(`{"key":"value"}`), models.EncryptedJSON(`{"key":"value"}`)},
		{"string", `{"foo":"bar"}`, models.EncryptedJSON(`{"foo":"bar"}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e models.EncryptedJSON
			err := e.Scan(tt.value)
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}
			if string(e) != string(tt.want) {
				t.Errorf("Scan() = %s, want %s", string(e), string(tt.want))
			}
		})
	}
}

func TestS3Credentials(t *testing.T) {
	creds := models.S3Credentials{
		Endpoint:       "https://s3.example.com",
		Key:            "access-key",
		Secret:         "secret-key",
		Region:         "us-east-1",
		Bucket:         "my-bucket",
		Path:           "/backups",
		ForcePathStyle: true,
	}

	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result models.S3Credentials
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result.Endpoint != creds.Endpoint {
		t.Errorf("Endpoint = %s, want %s", result.Endpoint, creds.Endpoint)
	}
	if result.Key != creds.Key {
		t.Errorf("Key = %s, want %s", result.Key, creds.Key)
	}
	if result.Secret != creds.Secret {
		t.Errorf("Secret = %s, want %s", result.Secret, creds.Secret)
	}
	if result.Region != creds.Region {
		t.Errorf("Region = %s, want %s", result.Region, creds.Region)
	}
	if result.Bucket != creds.Bucket {
		t.Errorf("Bucket = %s, want %s", result.Bucket, creds.Bucket)
	}
	if result.Path != creds.Path {
		t.Errorf("Path = %s, want %s", result.Path, creds.Path)
	}
	if result.ForcePathStyle != creds.ForcePathStyle {
		t.Errorf("ForcePathStyle = %v, want %v", result.ForcePathStyle, creds.ForcePathStyle)
	}
}

func TestDropboxCredentials(t *testing.T) {
	creds := models.DropboxCredentials{
		Token: "dropbox-token",
	}

	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result models.DropboxCredentials
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result.Token != creds.Token {
		t.Errorf("Token = %s, want %s", result.Token, creds.Token)
	}
}

func TestJSON_UnknownTypeScan(t *testing.T) {
	var j models.JSON
	err := j.Scan(12345) // Unknown type - current impl doesn't error, just ignores
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	// Should remain empty since type wasn't recognized
	if len(j) != 0 {
		t.Errorf("expected empty JSON, got %s", string(j))
	}
}

func TestEncryptedJSON_UnknownTypeScan(t *testing.T) {
	var e models.EncryptedJSON
	err := e.Scan(12345) // Unknown type - current impl doesn't error, just ignores
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	// Should remain empty since type wasn't recognized
	if len(e) != 0 {
		t.Errorf("expected empty EncryptedJSON, got %s", string(e))
	}
}

func TestJSON_ToStringSlice_InvalidJSON(t *testing.T) {
	j := models.JSON(`{invalid json}`)
	_, err := j.ToStringSlice()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestEncryptedJSON_Decrypt_InvalidJSON(t *testing.T) {
	e := models.EncryptedJSON(`{invalid json}`)
	_, err := e.Decrypt()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBackupJob_BeforeCreate_WithExistingStatus(t *testing.T) {
	db := setupTestDB(t)

	backup := &models.Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}
	db.Create(backup)

	job := &models.BackupJob{
		ID:                "existing-job-id",
		BackupID:          backup.ID,
		StorageProviderID: "provider123",
		Status:            enums.BackupJobStatusRunning, // Pre-set status
	}

	err := db.Create(job).Error
	if err != nil {
		t.Fatalf("failed to create backup job: %v", err)
	}

	// Status should remain as running
	if job.Status != enums.BackupJobStatusRunning {
		t.Errorf("expected Status to be running, got %s", job.Status)
	}

	// ID should remain as existing-job-id
	if job.ID != "existing-job-id" {
		t.Errorf("expected ID to remain 'existing-job-id', got %s", job.ID)
	}
}

func TestStorageProvider_SetCredentials_EmptyMap(t *testing.T) {
	provider := &models.StorageProvider{}

	// Set empty map (not nil)
	err := provider.SetCredentials(map[string]interface{}{})
	if err != nil {
		t.Fatalf("SetCredentials() error = %v", err)
	}

	got, err := provider.GetCredentials()
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}

	// Should return empty map
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestBackupDatabase_BeforeCreate_WithExistingID(t *testing.T) {
	db := setupTestDB(t)

	backup := &models.Backup{
		ServerID:          "server123",
		UserID:            "user123",
		StorageProviderID: "provider123",
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
	}
	db.Create(backup)

	backupDB := &models.BackupDatabase{
		ID:         "existing-backupdb-id",
		BackupID:   backup.ID,
		DatabaseID: "db123",
	}

	err := db.Create(backupDB).Error
	if err != nil {
		t.Fatalf("failed to create backup database: %v", err)
	}

	// ID should remain as existing-backupdb-id
	if backupDB.ID != "existing-backupdb-id" {
		t.Errorf("expected ID to remain 'existing-backupdb-id', got %s", backupDB.ID)
	}
}
