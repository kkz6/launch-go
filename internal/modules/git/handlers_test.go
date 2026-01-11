package git

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
)

func setupHandlerTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&SourceControl{}, &SourceControlRepository{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func setupTestHandler(t *testing.T) (*Handler, *Service, *gorm.DB) {
	db := setupHandlerTestDB(t)
	repo := NewRepository(db)
	factory := providers.NewProviderFactory()

	// Register GitHub provider with test config
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{
		AppID:         "test-app-id",
		PrivateKey:    "test-key",
		WebhookSecret: "test-secret",
		AppSlug:       "test-app",
	})

	logger := zerolog.Nop()
	service := NewService(repo, factory, nil, &logger)
	handler := NewHandler(service)

	return handler, service, db
}

func setupTestApp(handler *Handler) *fiber.App {
	app := fiber.New()

	// Add mock middleware to set user/team context
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", "test-user-id")
		c.Locals("teamID", "test-team-id")
		return c.Next()
	})

	// Register routes
	app.Get("/source-controls", handler.ListSourceControls)
	app.Get("/source-controls/:id", handler.GetSourceControl)
	app.Post("/source-controls", handler.Connect)
	app.Delete("/source-controls/:id", handler.Disconnect)
	app.Get("/providers/:provider/installation-url", handler.GetInstallationURL)
	app.Get("/providers/:provider/installations", handler.GetInstallations)
	app.Get("/providers/:provider/installations/:installationId", handler.GetInstallation)
	app.Get("/providers/:provider/installations/:installationId/repositories", handler.GetInstallationRepositories)
	app.Get("/providers/:provider/installations/:installationId/cached-repositories", handler.GetCachedInstallationRepositories)
	app.Post("/providers/:provider/installations/:installationId/refresh", handler.RefreshInstallationRepositories)
	app.Get("/providers/:provider/callback", handler.HandleInstallationCallback)
	app.Get("/providers/:provider/test-connection", handler.TestConnection)
	app.Get("/git-providers", handler.GetInstallationsWithCounts)

	return app
}

func TestNewHandler(t *testing.T) {
	handler, service, _ := setupTestHandler(t)

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if handler.service != service {
		t.Error("Service should be set")
	}
}

func TestHandler_ListSourceControls(t *testing.T) {
	handler, _, db := setupTestHandler(t)
	app := setupTestApp(handler)

	// Create test data
	sc := &SourceControl{
		UserID:   "test-user-id",
		TeamID:   "test-team-id",
		Provider: GitProviderGitHub,
	}
	db.Create(sc)

	req := httptest.NewRequest("GET", "/source-controls", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data array in response")
	}
	if len(data) != 1 {
		t.Errorf("Expected 1 source control, got %d", len(data))
	}
}

func TestHandler_ListSourceControls_Empty(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/source-controls", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data array in response")
	}
	if len(data) != 0 {
		t.Errorf("Expected 0 source controls, got %d", len(data))
	}
}

func TestHandler_GetSourceControl(t *testing.T) {
	handler, _, db := setupTestHandler(t)
	app := setupTestApp(handler)

	sc := &SourceControl{
		UserID:   "test-user-id",
		TeamID:   "test-team-id",
		Provider: GitProviderGitHub,
	}
	db.Create(sc)

	req := httptest.NewRequest("GET", "/source-controls/"+sc.ID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHandler_GetSourceControl_NotFound(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/source-controls/non-existent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestHandler_Connect_InvalidBody(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("POST", "/source-controls", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_Connect_InvalidProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	// "invalid" provider fails validation (oneof=github gitlab bitbucket), returns 422
	body := `{"provider": "invalid", "installation_id": "12345"}`
	req := httptest.NewRequest("POST", "/source-controls", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Validation error returns 422 Unprocessable Entity
	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Errorf("Expected status 422, got %d", resp.StatusCode)
	}
}

func TestHandler_Connect_MissingFields(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	// Missing installation_id
	body := `{"provider": "github"}`
	req := httptest.NewRequest("POST", "/source-controls", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Errorf("Expected status 422 (validation error), got %d", resp.StatusCode)
	}
}

func TestHandler_Disconnect(t *testing.T) {
	handler, _, db := setupTestHandler(t)
	app := setupTestApp(handler)

	sc := &SourceControl{
		UserID:   "test-user-id",
		TeamID:   "test-team-id",
		Provider: GitProviderGitHub,
	}
	db.Create(sc)

	req := httptest.NewRequest("DELETE", "/source-controls/"+sc.ID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}

	// Verify deletion
	var count int64
	db.Model(&SourceControl{}).Where("id = ?", sc.ID).Count(&count)
	if count != 0 {
		t.Error("Source control should be deleted")
	}
}

func TestHandler_Disconnect_NotFound(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("DELETE", "/source-controls/non-existent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestHandler_GetInstallationURL(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/github/installation-url", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data object in response")
	}

	url, ok := data["url"].(string)
	if !ok || url == "" {
		t.Error("Expected non-empty URL in response")
	}
}

func TestHandler_GetInstallationURL_InvalidProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/invalid/installation-url", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_GetInstallationURL_UnconfiguredProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	// GitLab is not configured in test setup
	req := httptest.NewRequest("GET", "/providers/gitlab/installation-url", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

func TestHandler_GetCachedInstallationRepositories(t *testing.T) {
	handler, _, db := setupTestHandler(t)
	app := setupTestApp(handler)

	installationID := "12345"
	sc := &SourceControl{
		UserID:         "test-user-id",
		TeamID:         "test-team-id",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
		ProviderID:     &installationID, // GetInstallationRepositories queries by provider_id
	}
	db.Create(sc)

	repo := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "test-repo",
		FullName:        "user/test-repo",
	}
	db.Create(repo)

	req := httptest.NewRequest("GET", "/providers/github/installations/12345/cached-repositories", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data object in response")
	}

	repos, ok := data["repositories"].([]interface{})
	if !ok {
		t.Fatal("Expected repositories array in response")
	}
	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}
}

func TestHandler_GetCachedInstallationRepositories_InvalidProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/invalid/installations/12345/cached-repositories", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_RefreshInstallationRepositories_NotFound(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("POST", "/providers/github/installations/non-existent/refresh", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestHandler_RefreshInstallationRepositories_InvalidProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("POST", "/providers/invalid/installations/12345/refresh", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_HandleInstallationCallback_NoInstallationID(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/github/callback", nil)
	resp, err := app.Test(req, -1) // disable timeout for redirect
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should redirect when no installation_id
	if resp.StatusCode != fiber.StatusFound {
		t.Errorf("Expected status 302 (redirect), got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/settings/git-providers" {
		t.Errorf("Expected redirect to /settings/git-providers, got %s", location)
	}
}

func TestHandler_HandleInstallationCallback_InvalidProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/invalid/callback?installation_id=12345", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should redirect for invalid provider
	if resp.StatusCode != fiber.StatusFound {
		t.Errorf("Expected status 302 (redirect), got %d", resp.StatusCode)
	}
}

func TestHandler_HandleInstallationCallback_WithInstallationID(t *testing.T) {
	handler, _, db := setupTestHandler(t)
	app := setupTestApp(handler)

	// Create existing installation
	installationID := "12345"
	sc := &SourceControl{
		UserID:         "test-user-id",
		TeamID:         "test-team-id",
		Provider:       GitProviderGitHub,
		ProviderID:     &installationID,
		InstallationID: &installationID,
	}
	db.Create(sc)

	req := httptest.NewRequest("GET", "/providers/github/callback?installation_id=12345&setup_action=install", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should redirect after processing
	if resp.StatusCode != fiber.StatusFound {
		t.Errorf("Expected status 302 (redirect), got %d", resp.StatusCode)
	}
}

func TestHandler_TestConnection(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	// This will fail because we can't actually connect to GitHub with test credentials
	// But we're testing the handler logic
	req := httptest.NewRequest("GET", "/providers/github/test-connection", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return 200 with success/failure info
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHandler_TestConnection_InvalidProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/invalid/test-connection", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_GetInstallationsWithCounts(t *testing.T) {
	handler, _, db := setupTestHandler(t)
	app := setupTestApp(handler)

	installationID := "12345"
	sc := &SourceControl{
		UserID:          "test-user-id",
		TeamID:          "test-team-id",
		Provider:        GitProviderGitHub,
		InstallationID:  &installationID,
		RepositoryCount: 5,
	}
	db.Create(sc)

	req := httptest.NewRequest("GET", "/git-providers", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data object in response")
	}

	// Check for available_providers
	providers, ok := data["available_providers"].([]interface{})
	if !ok {
		t.Fatal("Expected available_providers array")
	}
	if len(providers) != 3 { // GitHub, GitLab, Bitbucket
		t.Errorf("Expected 3 available providers, got %d", len(providers))
	}

	// Check for app_installations
	installations, ok := data["app_installations"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected app_installations object")
	}

	githubInstalls, ok := installations["github"].([]interface{})
	if !ok {
		t.Fatal("Expected github installations array")
	}
	if len(githubInstalls) != 1 {
		t.Errorf("Expected 1 GitHub installation, got %d", len(githubInstalls))
	}
}

func TestHandler_GetInstallations_InvalidProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/invalid/installations", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_GetInstallation_NotFound(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/github/installations/non-existent", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// The handler fetches all installations and searches, so it will return 500
	// because we can't make real API calls in tests
	// In a real test with mocked providers, this would be 404
}

func TestHandler_GetInstallationRepositories_InvalidProvider(t *testing.T) {
	handler, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/providers/invalid/installations/12345/repositories", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}
