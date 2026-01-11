package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
)

func setupWebhookTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&SourceControl{}, &SourceControlRepository{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func setupTestWebhookHandler(t *testing.T) (*WebhookHandler, *Service, *providers.ProviderFactory, *gorm.DB) {
	db := setupWebhookTestDB(t)
	repo := NewRepository(db)
	factory := providers.NewProviderFactory()

	// Register GitHub provider with test config
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{
		AppID:         "test-app-id",
		PrivateKey:    "test-key",
		WebhookSecret: "test-webhook-secret",
		AppSlug:       "test-app",
	})

	// Register GitLab provider
	factory.RegisterConfig(providers.GitProviderGitLab, &providers.ProviderConfig{
		ClientID:      "test-client-id",
		ClientSecret:  "test-secret",
		WebhookSecret: "test-gitlab-secret",
	})

	// Register Bitbucket provider
	factory.RegisterConfig(providers.GitProviderBitbucket, &providers.ProviderConfig{
		ClientID:      "test-client-id",
		ClientSecret:  "test-secret",
		WebhookSecret: "test-bitbucket-secret",
	})

	logger := zerolog.Nop()
	service := NewService(repo, factory, nil, &logger)
	webhookHandler := NewWebhookHandler(service, factory, &logger)

	return webhookHandler, service, factory, db
}

func setupWebhookTestApp(handler *WebhookHandler) *fiber.App {
	app := fiber.New()
	app.Post("/webhooks/:provider", handler.HandleWebhook)
	return app
}

func generateGitHubSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func generateBitbucketSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestNewWebhookHandler(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)

	if handler == nil {
		t.Fatal("NewWebhookHandler() returned nil")
	}
}

func TestWebhookHandler_HandleWebhook_InvalidProvider(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := `{"action": "created"}`
	req := httptest.NewRequest("POST", "/webhooks/invalid", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HandleWebhook_MissingSignature_GitHub(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := `{"action": "created"}`
	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	// No signature header
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HandleWebhook_InvalidSignature_GitHub(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := `{"action": "created"}`
	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HandleWebhook_ValidSignature_GitHub(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := []byte(`{"action": "created", "installation": {"id": 12345}}`)
	signature := generateGitHubSignature("test-webhook-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HandleWebhook_InvalidJSON(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := []byte(`invalid json`)
	signature := generateGitHubSignature("test-webhook-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HandleWebhook_GitLab(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := []byte(`{"event_type": "push", "project": {"path_with_namespace": "user/repo"}}`)

	req := httptest.NewRequest("POST", "/webhooks/gitlab", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Token", "test-gitlab-secret")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HandleWebhook_GitLab_InvalidToken(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := []byte(`{"event_type": "push"}`)

	req := httptest.NewRequest("POST", "/webhooks/gitlab", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Token", "wrong-secret")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HandleWebhook_Bitbucket(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := []byte(`{"push": {"changes": []}}`)
	signature := generateBitbucketSignature("test-bitbucket-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/bitbucket", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hook-UUID", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestWebhookHandler_getSignature was removed because it tested an unexported method

func TestWebhookHandler_GitHub_InstallationCreated(t *testing.T) {
	handler, _, _, db := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	// Create existing source control
	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		ProviderID:     &installationID,
		InstallationID: &installationID,
	}
	db.Create(sc)

	payload := []byte(`{
		"action": "created",
		"installation": {
			"id": 12345,
			"account": {
				"login": "testuser",
				"type": "User"
			}
		},
		"sender": {
			"login": "installer",
			"id": 999,
			"type": "User",
			"avatar_url": "https://example.com/avatar.png"
		}
	}`)
	signature := generateGitHubSignature("test-webhook-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GitHub_InstallationDeleted(t *testing.T) {
	handler, _, _, db := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	// Create existing source control
	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
	}
	db.Create(sc)

	payload := []byte(`{
		"action": "deleted",
		"installation": {
			"id": 12345
		}
	}`)
	signature := generateGitHubSignature("test-webhook-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GitHub_RepositoriesAdded(t *testing.T) {
	handler, _, _, db := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
	}
	db.Create(sc)

	payload := []byte(`{
		"action": "repositories_added",
		"installation": {
			"id": 12345
		},
		"repositories_added": [
			{"id": 1, "name": "repo1", "full_name": "user/repo1"}
		]
	}`)
	signature := generateGitHubSignature("test-webhook-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GitHub_PushEvent(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := []byte(`{
		"ref": "refs/heads/main",
		"repository": {
			"full_name": "user/repo"
		},
		"commits": [
			{
				"id": "abc123",
				"message": "Test commit",
				"author": {
					"name": "Test User",
					"email": "test@example.com"
				}
			}
		],
		"head_commit": {
			"id": "abc123",
			"message": "Test commit"
		}
	}`)
	signature := generateGitHubSignature("test-webhook-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GitLab_PushEvent(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := []byte(`{
		"event_type": "push",
		"ref": "refs/heads/main",
		"project": {
			"path_with_namespace": "user/repo"
		},
		"commits": [
			{
				"id": "abc123",
				"message": "Test commit"
			}
		]
	}`)

	req := httptest.NewRequest("POST", "/webhooks/gitlab", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Token", "test-gitlab-secret")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_Bitbucket_PushEvent(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	payload := []byte(`{
		"push": {
			"changes": [
				{
					"new": {
						"name": "main",
						"target": {
							"hash": "abc123"
						}
					}
				}
			]
		},
		"repository": {
			"full_name": "user/repo"
		}
	}`)
	signature := generateBitbucketSignature("test-bitbucket-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/bitbucket", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hook-UUID", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestFormatFloat was removed because it tested an unexported helper function

func TestWebhookHandler_UnconfiguredProvider(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewRepository(db)
	factory := providers.NewProviderFactory()

	// Don't register any providers

	logger := zerolog.Nop()
	service := NewService(repo, factory, nil, &logger)
	webhookHandler := NewWebhookHandler(service, factory, &logger)

	app := fiber.New()
	app.Post("/webhooks/:provider", webhookHandler.HandleWebhook)

	payload := []byte(`{"action": "created"}`)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=abc123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GitHub_EmptyInstallationID(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	// Installation with non-numeric ID should be handled gracefully
	payload := []byte(`{
		"action": "created",
		"installation": {
			"id": null
		}
	}`)
	signature := generateGitHubSignature("test-webhook-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should still return 200 (webhook received)
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GitHub_NoInstallation(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	// Webhook without installation data
	payload := []byte(`{
		"action": "opened",
		"pull_request": {
			"id": 123
		}
	}`)
	signature := generateGitHubSignature("test-webhook-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_GitLab_NonPushEvent(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	// Non-push event
	payload := []byte(`{
		"event_type": "merge_request",
		"project": {
			"path_with_namespace": "user/repo"
		}
	}`)

	req := httptest.NewRequest("POST", "/webhooks/gitlab", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Token", "test-gitlab-secret")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_Bitbucket_NoPushData(t *testing.T) {
	handler, _, _, _ := setupTestWebhookHandler(t)
	app := setupWebhookTestApp(handler)

	// Webhook without push data
	payload := []byte(`{
		"pullrequest": {
			"id": 123
		}
	}`)
	signature := generateBitbucketSignature("test-bitbucket-secret", payload)

	req := httptest.NewRequest("POST", "/webhooks/bitbucket", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hook-UUID", signature)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
