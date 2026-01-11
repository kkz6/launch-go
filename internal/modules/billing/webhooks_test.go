package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/handlers"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testWebhookSecret = "test-webhook-secret"

func setupTestWebhookHandler(t *testing.T) (*handlers.WebhookHandler, *fiber.App, *repositories.BillingRepository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Subscription{}, &models.Order{}, &models.WebhookEvent{})
	require.NoError(t, err)

	repo := repositories.NewBillingRepository(db)
	log := zerolog.Nop()

	config := &services.Config{
		SubscriptionsEnabled: true,
		Plans:                models.DefaultPlans(),
	}

	service := services.NewBillingService(repo, nil, config, &log)
	webhookHandler := handlers.NewWebhookHandler(repo, service, testWebhookSecret, &log)

	app := fiber.New()

	return webhookHandler, app, repo, db
}

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func createWebhookPayload(eventName string, teamID string, subscriptionID string) []byte {
	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": eventName,
			"custom_data": map[string]string{
				"team_id": teamID,
			},
			"test_mode": true,
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   subscriptionID,
			"attributes": map[string]interface{}{
				"store_id":       1,
				"customer_id":    123,
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test Plan",
				"variant_name":   "Monthly",
				"status":         "active",
				"billing_anchor": 1,
				"created_at":     "2024-01-01T00:00:00Z",
				"updated_at":     "2024-01-01T00:00:00Z",
			},
		},
	}

	data, _ := json.Marshal(payload)
	return data
}

func createOrderWebhookPayload(eventName string, teamID string, orderID string) []byte {
	orderNum := 12345
	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": eventName,
			"custom_data": map[string]string{
				"team_id": teamID,
			},
			"test_mode": true,
		},
		"data": map[string]interface{}{
			"type": "orders",
			"id":   orderID,
			"attributes": map[string]interface{}{
				"store_id":       1,
				"customer_id":    123,
				"product_id":     1,
				"variant_id":     1,
				"order_number":   orderNum,
				"currency":       "USD",
				"currency_rate":  "1.0",
				"subtotal":       1000,
				"discount_total": 0,
				"tax":            100,
				"total":          1100,
				"status":         "paid",
				"created_at":     "2024-01-01T00:00:00Z",
				"updated_at":     "2024-01-01T00:00:00Z",
			},
		},
	}

	data, _ := json.Marshal(payload)
	return data
}

func TestWebhookHandler_HandleWebhook_MissingSignature(t *testing.T) {
	webhookHandler, app, _, _ := setupTestWebhookHandler(t)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := createWebhookPayload("subscription_created", "team_1", "ls_123")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWebhookHandler_HandleWebhook_InvalidSignature(t *testing.T) {
	webhookHandler, app, _, _ := setupTestWebhookHandler(t)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := createWebhookPayload("subscription_created", "team_1", "ls_123")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", "invalid-signature")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWebhookHandler_HandleWebhook_InvalidPayload(t *testing.T) {
	webhookHandler, app, _, _ := setupTestWebhookHandler(t)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := []byte("invalid json")
	signature := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebhookHandler_HandleWebhook_SubscriptionCreated(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := createWebhookPayload("subscription_created", "team_1", "ls_123")
	signature := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	subscriptions, err := repo.FindSubscriptionsByTeam(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Len(t, subscriptions, 1)
	assert.Equal(t, "ls_123", subscriptions[0].LemonSqueezyID)
}

func TestWebhookHandler_HandleWebhook_SubscriptionUpdated(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "1",
		VariantID:      "1",
		Name:           "Old Name",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := createWebhookPayload("subscription_updated", "team_1", "ls_123")
	signature := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	updated, err := repo.FindSubscriptionByLemonSqueezyID(context.Background(), "ls_123")
	assert.NoError(t, err)
	assert.Equal(t, "Test Plan", updated.Name)
}

func TestWebhookHandler_HandleWebhook_SubscriptionCancelled(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "1",
		VariantID:      "1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	endsAt := time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": "subscription_cancelled",
			"custom_data": map[string]string{
				"team_id": "team_1",
			},
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   "ls_123",
			"attributes": map[string]interface{}{
				"status":         "cancelled",
				"ends_at":        endsAt,
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test",
				"billing_anchor": 1,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	signature := signPayload(payloadBytes, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	updated, err := repo.FindSubscriptionByLemonSqueezyID(context.Background(), "ls_123")
	assert.NoError(t, err)
	assert.Equal(t, enums.SubscriptionStatusCancelled, updated.Status)
	assert.NotNil(t, updated.EndsAt)
}

func TestWebhookHandler_HandleWebhook_SubscriptionResumed(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	endsAt := time.Now().Add(7 * 24 * time.Hour)
	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "1",
		VariantID:      "1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusCancelled,
		EndsAt:         &endsAt,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	renewsAt := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": "subscription_resumed",
			"custom_data": map[string]string{
				"team_id": "team_1",
			},
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   "ls_123",
			"attributes": map[string]interface{}{
				"status":         "active",
				"renews_at":      renewsAt,
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test",
				"billing_anchor": 1,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	signature := signPayload(payloadBytes, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	updated, err := repo.FindSubscriptionByLemonSqueezyID(context.Background(), "ls_123")
	assert.NoError(t, err)
	assert.Equal(t, enums.SubscriptionStatusActive, updated.Status)
	assert.Nil(t, updated.EndsAt)
}

func TestWebhookHandler_HandleWebhook_SubscriptionPaused(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "1",
		VariantID:      "1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	resumesAt := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": "subscription_paused",
			"custom_data": map[string]string{
				"team_id": "team_1",
			},
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   "ls_123",
			"attributes": map[string]interface{}{
				"status":         "paused",
				"resumes_at":     resumesAt,
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test",
				"billing_anchor": 1,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	signature := signPayload(payloadBytes, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	updated, err := repo.FindSubscriptionByLemonSqueezyID(context.Background(), "ls_123")
	assert.NoError(t, err)
	assert.Equal(t, enums.SubscriptionStatusPaused, updated.Status)
	assert.NotNil(t, updated.PausedAt)
}

func TestWebhookHandler_HandleWebhook_OrderCreated(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := createOrderWebhookPayload("order_created", "team_1", "ls_order_123")
	signature := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	orders, err := repo.FindOrdersByTeam(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, "ls_order_123", orders[0].LemonSqueezyID)
	assert.Equal(t, int64(1100), orders[0].Total)
}

func TestWebhookHandler_HandleWebhook_OrderRefunded(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	order := &models.Order{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_order_123",
		CustomerID:     "cust_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		OrderNumber:    "123",
		Currency:       "USD",
		CurrencyRate:   "1.0",
		Subtotal:       1000,
		Total:          1000,
		Status:         enums.OrderStatusPaid,
	}
	repo.CreateOrder(context.Background(), order)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := createOrderWebhookPayload("order_refunded", "team_1", "ls_order_123")
	signature := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	updated, err := repo.FindOrderByLemonSqueezyID(context.Background(), "ls_order_123")
	assert.NoError(t, err)
	assert.Equal(t, enums.OrderStatusRefunded, updated.Status)
	assert.NotNil(t, updated.RefundedAt)
}

func TestWebhookHandler_HandleWebhook_UnknownEvent(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := createWebhookPayload("unknown_event", "team_1", "ls_123")
	signature := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	events, err := repo.FindUnprocessedWebhookEvents(context.Background(), 10)
	assert.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestWebhookHandler_HandleWebhook_MissingTeamID(t *testing.T) {
	webhookHandler, app, _, _ := setupTestWebhookHandler(t)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": "subscription_created",
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   "ls_123",
			"attributes": map[string]interface{}{
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test",
				"status":         "active",
				"billing_anchor": 1,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	signature := signPayload(payloadBytes, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWebhookHandler_ProcessPendingWebhooks(t *testing.T) {
	webhookHandler, _, repo, _ := setupTestWebhookHandler(t)

	payload := createWebhookPayload("subscription_created", "team_1", "ls_pending_123")
	event := &models.WebhookEvent{
		EventName:  enums.WebhookEventSubscriptionCreated,
		Payload:    string(payload),
		Signature:  "sig",
		Processed:  false,
		RetryCount: 0,
	}
	repo.CreateWebhookEvent(context.Background(), event)

	err := webhookHandler.ProcessPendingWebhooks(context.Background())
	assert.NoError(t, err)

	processed, err := repo.FindWebhookEventByID(context.Background(), event.ID)
	assert.NoError(t, err)
	assert.True(t, processed.Processed)
}

func TestWebhookHandler_CleanupOldWebhookEvents(t *testing.T) {
	webhookHandler, _, repo, db := setupTestWebhookHandler(t)

	oldTime := time.Now().Add(-48 * time.Hour)
	event := &models.WebhookEvent{
		EventName: enums.WebhookEventSubscriptionCreated,
		Payload:   "{}",
		Signature: "sig",
		Processed: true,
	}
	repo.CreateWebhookEvent(context.Background(), event)
	db.Model(&models.WebhookEvent{}).Where("id = ?", event.ID).Update("processed_at", oldTime)

	err := webhookHandler.CleanupOldWebhookEvents(context.Background(), 24*time.Hour)
	assert.NoError(t, err)

	_, err = repo.FindWebhookEventByID(context.Background(), event.ID)
	assert.Error(t, err)
}

func TestWebhookHandler_VerifySignature(t *testing.T) {
	webhookHandler, _, _, _ := setupTestWebhookHandler(t)

	payload := []byte("test payload")
	validSignature := signPayload(payload, testWebhookSecret)

	assert.True(t, webhookHandler.VerifySignature(payload, validSignature))
	assert.False(t, webhookHandler.VerifySignature(payload, "invalid"))
	assert.False(t, webhookHandler.VerifySignature([]byte("different"), validSignature))
}

func TestMapLemonSqueezyStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected enums.SubscriptionStatus
	}{
		{"on_trial", enums.SubscriptionStatusOnTrial},
		{"active", enums.SubscriptionStatusActive},
		{"paused", enums.SubscriptionStatusPaused},
		{"past_due", enums.SubscriptionStatusPastDue},
		{"unpaid", enums.SubscriptionStatusUnpaid},
		{"cancelled", enums.SubscriptionStatusCancelled},
		{"expired", enums.SubscriptionStatusExpired},
		{"unknown", enums.SubscriptionStatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := handlers.MapLemonSqueezyStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTime(t *testing.T) {
	validTime := "2024-01-15T10:30:00Z"
	invalidTime := "invalid"
	emptyTime := ""

	result := handlers.ParseTime(&validTime)
	assert.NotNil(t, result)

	result = handlers.ParseTime(&invalidTime)
	assert.Nil(t, result)

	result = handlers.ParseTime(&emptyTime)
	assert.Nil(t, result)

	result = handlers.ParseTime(nil)
	assert.Nil(t, result)
}

func TestWebhookHandler_SubscriptionPaymentSuccess(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "1",
		VariantID:      "1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusPastDue,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	renewsAt := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": "subscription_payment_success",
			"custom_data": map[string]string{
				"team_id": "team_1",
			},
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   "ls_123",
			"attributes": map[string]interface{}{
				"status":         "active",
				"renews_at":      renewsAt,
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test",
				"billing_anchor": 1,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	signature := signPayload(payloadBytes, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	updated, err := repo.FindSubscriptionByLemonSqueezyID(context.Background(), "ls_123")
	assert.NoError(t, err)
	assert.Equal(t, enums.SubscriptionStatusActive, updated.Status)
}

func TestWebhookHandler_SubscriptionPaymentFailed(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "1",
		VariantID:      "1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": "subscription_payment_failed",
			"custom_data": map[string]string{
				"team_id": "team_1",
			},
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   "ls_123",
			"attributes": map[string]interface{}{
				"status":         "past_due",
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test",
				"billing_anchor": 1,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	signature := signPayload(payloadBytes, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWebhookHandler_SubscriptionExpired(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "1",
		VariantID:      "1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusCancelled,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": "subscription_expired",
			"custom_data": map[string]string{
				"team_id": "team_1",
			},
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   "ls_123",
			"attributes": map[string]interface{}{
				"status":         "expired",
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test",
				"billing_anchor": 1,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	signature := signPayload(payloadBytes, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWebhookHandler_SubscriptionUnpaused(t *testing.T) {
	webhookHandler, app, repo, _ := setupTestWebhookHandler(t)

	pausedAt := time.Now().Add(-24 * time.Hour)
	resumesAt := time.Now().Add(7 * 24 * time.Hour)
	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "1",
		VariantID:      "1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusPaused,
		PausedAt:       &pausedAt,
		ResumesAt:      &resumesAt,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)

	payload := map[string]interface{}{
		"meta": map[string]interface{}{
			"event_name": "subscription_unpaused",
			"custom_data": map[string]string{
				"team_id": "team_1",
			},
		},
		"data": map[string]interface{}{
			"type": "subscriptions",
			"id":   "ls_123",
			"attributes": map[string]interface{}{
				"status":         "active",
				"product_id":     1,
				"variant_id":     1,
				"product_name":   "Test",
				"billing_anchor": 1,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	signature := signPayload(payloadBytes, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/lemon-squeezy", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	updated, err := repo.FindSubscriptionByLemonSqueezyID(context.Background(), "ls_123")
	assert.NoError(t, err)
	assert.Equal(t, enums.SubscriptionStatusActive, updated.Status)
	assert.Nil(t, updated.PausedAt)
	assert.Nil(t, updated.ResumesAt)
}
