package backup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/handlers"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/services"
)

func setupTestHandler(t *testing.T) (*handlers.BackupHandler, *handlers.StorageProviderHandler, *services.BackupService, *services.StorageProviderService, *fiber.App) {
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

	backupHandler := handlers.NewBackupHandler(backupService)
	backupJobHandler := handlers.NewBackupJobHandler(backupJobService)
	storageProviderHandler := handlers.NewStorageProviderHandler(storageProviderService)

	app := fiber.New()

	// Mock middleware to set user and team IDs
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", "user123")
		c.Locals("teamID", "team123")
		return c.Next()
	})

	// Register routes
	app.Get("/servers/:serverId/backups", backupHandler.ListBackups)
	app.Post("/servers/:serverId/backups", backupHandler.CreateBackup)
	app.Get("/servers/:serverId/backups/:id", backupHandler.ShowBackup)
	app.Put("/servers/:serverId/backups/:id", backupHandler.UpdateBackup)
	app.Delete("/servers/:serverId/backups/:id", backupHandler.DeleteBackup)
	app.Post("/servers/:serverId/backups/:id/run", backupHandler.RunManualBackup)

	app.Post("/backup/:backup/:token", backupJobHandler.CreateBackupJob)

	app.Get("/storage-providers", storageProviderHandler.ListStorageProviders)
	app.Get("/storage-providers/dropdown", storageProviderHandler.ListStorageProvidersForDropdown)
	app.Get("/storage-providers/:id", storageProviderHandler.ShowStorageProvider)
	app.Post("/storage-providers/:provider/connect", storageProviderHandler.ConnectStorageProvider)
	app.Put("/storage-providers/:provider", storageProviderHandler.UpdateStorageProvider)
	app.Delete("/storage-providers/:provider", storageProviderHandler.DeleteStorageProvider)

	return backupHandler, storageProviderHandler, backupService, storageProviderService, app
}

func makeRequest(app *fiber.App, method, url string, body interface{}) (*http.Response, []byte) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(jsonBody)
	}

	req := httptest.NewRequest(method, url, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)
	respBody, _ := io.ReadAll(resp.Body)

	return resp, respBody
}

func TestHandler_ListBackups(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	// Create some backups
	req := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	}
	backupService.CreateBackup(t.Context(), "server1", "user123", req)
	backupService.CreateBackup(t.Context(), "server1", "user123", req)

	resp, body := makeRequest(app, "GET", "/servers/server1/backups", nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data := result["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("expected 2 backups, got %d", len(data))
	}
}

func TestHandler_CreateBackup(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	req := map[string]interface{}{
		"cron_expression":     "0 0 * * *",
		"path":                "/var/www",
		"enabled":             true,
		"database":            "01HXYZ123456789ABCDEF",
		"storage_provider_id": "provider123",
	}

	resp, body := makeRequest(app, "POST", "/servers/server1/backups", req)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusCreated, string(body))
	}
}

func TestHandler_CreateBackup_ValidationError(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	req := map[string]interface{}{
		"path": "/var/www",
		// Missing required fields
	}

	resp, _ := makeRequest(app, "POST", "/servers/server1/backups", req)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestHandler_ShowBackup(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	backup, _ := backupService.CreateBackup(t.Context(), "server1", "user123", &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	})

	resp, body := makeRequest(app, "GET", fmt.Sprintf("/servers/server1/backups/%s", backup.ID), nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

func TestHandler_ShowBackup_NotFound(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	resp, _ := makeRequest(app, "GET", "/servers/server1/backups/nonexistent", nil)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandler_UpdateBackup(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	backup, _ := backupService.CreateBackup(t.Context(), "server1", "user123", &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	})

	updateReq := map[string]interface{}{
		"cron_expression":     "0 12 * * *",
		"path":                "/new/path",
		"enabled":             false,
		"database":            "01HXYZ123456789ABCDEF",
		"storage_provider_id": "provider456",
	}

	resp, body := makeRequest(app, "PUT", fmt.Sprintf("/servers/server1/backups/%s", backup.ID), updateReq)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

func TestHandler_UpdateBackup_NotFound(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	updateReq := map[string]interface{}{
		"cron_expression":     "0 0 * * *",
		"path":                "/path",
		"enabled":             true,
		"database":            "01HXYZ123456789ABCDEF",
		"storage_provider_id": "provider123",
	}

	resp, _ := makeRequest(app, "PUT", "/servers/server1/backups/nonexistent", updateReq)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandler_DeleteBackup(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	backup, _ := backupService.CreateBackup(t.Context(), "server1", "user123", &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	})

	resp, _ := makeRequest(app, "DELETE", fmt.Sprintf("/servers/server1/backups/%s", backup.ID), nil)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestHandler_DeleteBackup_NotFound(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	resp, _ := makeRequest(app, "DELETE", "/servers/server1/backups/nonexistent", nil)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandler_RunManualBackup(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	backup, _ := backupService.CreateBackup(t.Context(), "server1", "user123", &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	})

	resp, body := makeRequest(app, "POST", fmt.Sprintf("/servers/server1/backups/%s/run", backup.ID), nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

func TestHandler_CreateBackupJob(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	backup, _ := backupService.CreateBackup(t.Context(), "server1", "user123", &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	})

	jobReq := map[string]interface{}{
		"status": "finished",
		"size":   1024,
	}

	resp, _ := makeRequest(app, "POST", fmt.Sprintf("/backup/%s/%s", backup.ID, backup.DispatchToken), jobReq)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestHandler_CreateBackupJob_InvalidToken(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	backup, _ := backupService.CreateBackup(t.Context(), "server1", "user123", &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	})

	jobReq := map[string]interface{}{
		"status": "finished",
	}

	resp, _ := makeRequest(app, "POST", fmt.Sprintf("/backup/%s/invalid-token", backup.ID), jobReq)

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestHandler_ListStorageProviders(t *testing.T) {
	_, _, _, storageProviderService, app := setupTestHandler(t)

	storageProviderService.ConnectStorageProvider(t.Context(), "user123", "team123", &dto.CreateStorageProviderRequest{
		Label:    "S3",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	})

	resp, body := makeRequest(app, "GET", "/storage-providers", nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

func TestHandler_ListStorageProvidersForDropdown(t *testing.T) {
	_, _, _, storageProviderService, app := setupTestHandler(t)

	storageProviderService.ConnectStorageProvider(t.Context(), "user123", "team123", &dto.CreateStorageProviderRequest{
		Label:    "S3",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	})

	resp, body := makeRequest(app, "GET", "/storage-providers/dropdown", nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

func TestHandler_ConnectStorageProvider(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	req := map[string]interface{}{
		"label":    "My S3",
		"provider": "s3",
		"key":      "access-key",
		"secret":   "secret-key",
		"region":   "us-east-1",
		"bucket":   "my-bucket",
	}

	resp, body := makeRequest(app, "POST", "/storage-providers/s3/connect", req)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusCreated, string(body))
	}
}

func TestHandler_ConnectStorageProvider_InvalidProvider(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	req := map[string]interface{}{
		"label":    "Invalid",
		"provider": "invalid",
	}

	resp, _ := makeRequest(app, "POST", "/storage-providers/invalid/connect", req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandler_ShowStorageProvider(t *testing.T) {
	_, _, _, storageProviderService, app := setupTestHandler(t)

	provider, _ := storageProviderService.ConnectStorageProvider(t.Context(), "user123", "team123", &dto.CreateStorageProviderRequest{
		Label:    "S3",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	})

	resp, body := makeRequest(app, "GET", fmt.Sprintf("/storage-providers/%d", provider.ID), nil)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

func TestHandler_ShowStorageProvider_NotFound(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	resp, _ := makeRequest(app, "GET", "/storage-providers/999999", nil)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandler_ShowStorageProvider_InvalidID(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	resp, _ := makeRequest(app, "GET", "/storage-providers/invalid", nil)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandler_UpdateStorageProvider(t *testing.T) {
	_, _, _, storageProviderService, app := setupTestHandler(t)

	provider, _ := storageProviderService.ConnectStorageProvider(t.Context(), "user123", "team123", &dto.CreateStorageProviderRequest{
		Label:    "S3",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	})

	req := map[string]interface{}{
		"id":       provider.ID,
		"label":    "Updated S3",
		"provider": "s3",
		"key":      "new-key",
		"secret":   "new-secret",
		"region":   "eu-west-1",
		"bucket":   "new-bucket",
	}

	resp, body := makeRequest(app, "PUT", "/storage-providers/s3", req)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

func TestHandler_UpdateStorageProvider_InvalidProvider(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	req := map[string]interface{}{
		"id":       1,
		"label":    "Test",
		"provider": "invalid",
	}

	resp, _ := makeRequest(app, "PUT", "/storage-providers/invalid", req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandler_DeleteStorageProvider(t *testing.T) {
	_, _, _, storageProviderService, app := setupTestHandler(t)

	provider, _ := storageProviderService.ConnectStorageProvider(t.Context(), "user123", "team123", &dto.CreateStorageProviderRequest{
		Label:    "S3",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	})

	resp, _ := makeRequest(app, "DELETE", fmt.Sprintf("/storage-providers/%d", provider.ID), nil)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestHandler_DeleteStorageProvider_NotFound(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	resp, _ := makeRequest(app, "DELETE", "/storage-providers/999999", nil)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandler_DeleteStorageProvider_InvalidID(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	resp, _ := makeRequest(app, "DELETE", "/storage-providers/invalid", nil)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandler_RunManualBackup_NotFound(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	resp, _ := makeRequest(app, "POST", "/servers/server1/backups/nonexistent/run", nil)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandler_CreateBackupJob_ValidationError(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	backup, _ := backupService.CreateBackup(t.Context(), "server1", "user123", &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	})

	// Empty body - missing required status
	jobReq := map[string]interface{}{}

	resp, _ := makeRequest(app, "POST", fmt.Sprintf("/backup/%s/%s", backup.ID, backup.DispatchToken), jobReq)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestHandler_UpdateBackup_ValidationError(t *testing.T) {
	_, _, backupService, _, app := setupTestHandler(t)

	backup, _ := backupService.CreateBackup(t.Context(), "server1", "user123", &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "/var/www",
		Enabled:           true,
		DatabaseID:        "db123",
		StorageProviderID: "provider123",
	})

	// Missing required fields
	updateReq := map[string]interface{}{
		"enabled": true,
	}

	resp, _ := makeRequest(app, "PUT", fmt.Sprintf("/servers/server1/backups/%s", backup.ID), updateReq)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestHandler_ConnectStorageProvider_ValidationError(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	// Missing required fields for s3
	req := map[string]interface{}{
		"label":    "My S3",
		"provider": "s3",
		// missing key, secret, region, bucket
	}

	resp, _ := makeRequest(app, "POST", "/storage-providers/s3/connect", req)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestHandler_UpdateStorageProvider_ValidationError(t *testing.T) {
	_, _, _, storageProviderService, app := setupTestHandler(t)

	provider, _ := storageProviderService.ConnectStorageProvider(t.Context(), "user123", "team123", &dto.CreateStorageProviderRequest{
		Label:    "S3",
		Provider: "s3",
		Key:      "key",
		Secret:   "secret",
		Region:   "us-east-1",
		Bucket:   "bucket",
	})

	// Missing required fields
	req := map[string]interface{}{
		"id":       provider.ID,
		"provider": "s3",
		// missing label, key, secret, etc.
	}

	resp, _ := makeRequest(app, "PUT", "/storage-providers/s3", req)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestHandler_UpdateStorageProvider_NotFound(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	req := map[string]interface{}{
		"id":       999999,
		"label":    "Test",
		"provider": "s3",
		"key":      "key",
		"secret":   "secret",
		"region":   "us-east-1",
		"bucket":   "bucket",
	}

	resp, _ := makeRequest(app, "PUT", "/storage-providers/s3", req)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandler_CreateBackupJob_BackupNotFound(t *testing.T) {
	_, _, _, _, app := setupTestHandler(t)

	jobReq := map[string]interface{}{
		"status": "finished",
		"size":   1024,
	}

	resp, _ := makeRequest(app, "POST", "/backup/nonexistent-backup/some-token", jobReq)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
