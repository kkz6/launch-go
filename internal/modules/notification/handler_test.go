package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/handlers"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
)

func setupTestHandler(t *testing.T) (*handlers.NotificationChannelHandler, *fiber.App, *repositories.NotificationChannelRepository) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(&models.NotificationChannel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	repo := repositories.NewNotificationChannelRepository(db)

	mockHTTP := &channels.MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			return nil, 200, nil
		},
	}
	factory := channels.NewFactory(mockHTTP)

	log := zerolog.Nop()
	service := services.NewNotificationChannelService(repo, factory, &log)
	handler := handlers.NewNotificationChannelHandler(service)

	app := fiber.New()

	// Mock auth middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", "user123")
		c.Locals("teamID", "team123")
		return c.Next()
	})

	app.Get("/notifications", handler.Index)
	app.Post("/notifications", handler.Store)
	app.Get("/notifications/:id", handler.Show)
	app.Put("/notifications/:id", handler.Update)
	app.Delete("/notifications/:id", handler.Destroy)
	app.Post("/notifications/:id/test", handler.Test)
	app.Post("/notifications/:id/default", handler.SetDefault)
	app.Post("/notifications/:id/disconnect", handler.Disconnect)
	app.Post("/notifications/:id/reconnect", handler.Reconnect)

	return handler, app, repo
}

func TestHandler_Index(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	// Create some channels
	chans := []models.NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user123", TeamID: "team123", Provider: enums.ChannelTypeEmail, Label: "Email 1"},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user123", TeamID: "team123", Provider: enums.ChannelTypeSlack, Label: "Slack 1"},
	}

	for _, ch := range chans {
		_ = repo.Create(ctx, &ch)
	}

	req := httptest.NewRequest("GET", "/notifications", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	if !response["success"].(bool) {
		t.Error("success should be true")
	}
}

func TestHandler_Show(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: enums.ChannelTypeEmail,
		Label:    "Test Email",
	}
	_ = repo.Create(ctx, channel)

	req := httptest.NewRequest("GET", "/notifications/01HXYZ123456789ABCDEFGHIJ", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestHandler_Show_NotFound(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/notifications/nonexistent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}
}

func TestHandler_Store(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	reqBody := dto.CreateChannelRequest{
		Provider:   "slack",
		Label:      "My Slack",
		WebhookURL: "https://hooks.slack.com/xxx",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/notifications", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want %d, body: %s", resp.StatusCode, fiber.StatusCreated, string(body))
	}
}

func TestHandler_Store_InvalidBody(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/notifications", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func TestHandler_Store_ValidationError(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	reqBody := dto.CreateChannelRequest{
		Provider: "email",
		// Missing required fields
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/notifications", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusUnprocessableEntity)
	}
}

func TestHandler_Store_InvalidProvider(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	reqBody := map[string]interface{}{
		"provider": "invalid",
		"label":    "Test",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/notifications", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	// Should fail validation since "invalid" is not a valid provider
	if resp.StatusCode != fiber.StatusUnprocessableEntity && resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want validation error or bad request", resp.StatusCode)
	}
}

func TestHandler_Update(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Original",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	reqBody := dto.UpdateChannelRequest{
		Label:      "Updated",
		WebhookURL: "https://hooks.slack.com/yyy",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/notifications/01HXYZ123456789ABCDEFGHIJ", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want %d, body: %s", resp.StatusCode, fiber.StatusOK, string(body))
	}
}

func TestHandler_Update_NotFound(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	reqBody := dto.UpdateChannelRequest{Label: "Test"}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/notifications/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}
}

func TestHandler_Destroy(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: enums.ChannelTypeEmail,
		Label:    "To Delete",
	}
	_ = repo.Create(ctx, channel)

	req := httptest.NewRequest("DELETE", "/notifications/01HXYZ123456789ABCDEFGHIJ", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNoContent)
	}
}

func TestHandler_Destroy_NotFound(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	req := httptest.NewRequest("DELETE", "/notifications/nonexistent", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}
}

func TestHandler_Test(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test Slack",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	reqBody := dto.TestChannelRequest{Message: "Test message"}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/notifications/01HXYZ123456789ABCDEFGHIJ/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want %d, body: %s", resp.StatusCode, fiber.StatusOK, string(body))
	}
}

func TestHandler_Test_DefaultMessage(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test Slack",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	// Send empty body - should use default message
	req := httptest.NewRequest("POST", "/notifications/01HXYZ123456789ABCDEFGHIJ/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestHandler_Test_NotFound(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/notifications/nonexistent/test", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}
}

func TestHandler_SetDefault(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: enums.ChannelTypeEmail,
		Label:    "Test Email",
	}
	_ = repo.Create(ctx, channel)

	req := httptest.NewRequest("POST", "/notifications/01HXYZ123456789ABCDEFGHIJ/default", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestHandler_SetDefault_NotFound(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/notifications/nonexistent/default", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}
}

func TestHandler_Disconnect(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeEmail,
		Label:     "Test Email",
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	req := httptest.NewRequest("POST", "/notifications/01HXYZ123456789ABCDEFGHIJ/disconnect", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestHandler_Disconnect_NotFound(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/notifications/nonexistent/disconnect", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}
}

func TestHandler_Reconnect(t *testing.T) {
	_, app, repo := setupTestHandler(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test Slack",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: false,
	}
	_ = repo.Create(ctx, channel)

	req := httptest.NewRequest("POST", "/notifications/01HXYZ123456789ABCDEFGHIJ/reconnect", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want %d, body: %s", resp.StatusCode, fiber.StatusOK, string(body))
	}
}

func TestHandler_Reconnect_NotFound(t *testing.T) {
	_, app, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/notifications/nonexistent/reconnect", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}
}

func TestNewHandler(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	repo := repositories.NewNotificationChannelRepository(db)
	factory := channels.NewFactory(&channels.MockHTTPClient{})
	log := zerolog.Nop()
	service := services.NewNotificationChannelService(repo, factory, &log)

	handler := handlers.NewNotificationChannelHandler(service)

	if handler == nil {
		t.Error("NewNotificationChannelHandler() returned nil")
	}
	if handler.GetService() != service {
		t.Error("service not set correctly")
	}
}
