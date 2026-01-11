package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/handlers"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
)

func setupTestHandler(t *testing.T) (*handlers.BillingHandler, *fiber.App, *repositories.BillingRepository, *gorm.DB) {
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

	serverCountFn := func(teamID string) (int, error) {
		return 2, nil
	}

	handler := handlers.NewBillingHandler(service, serverCountFn)

	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("teamID", "team_1")
		c.Locals("userID", "user_1")
		c.Locals("userRole", "customer")
		return c.Next()
	})

	return handler, app, repo, db
}

func TestHandler_Index(t *testing.T) {
	handler, app, repo, _ := setupTestHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	order := &models.Order{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_order_1",
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

	app.Get("/billing", handler.Index)

	req := httptest.NewRequest(http.MethodGet, "/billing", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	assert.NotNil(t, result["data"])
}

func TestHandler_GetPlans(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Get("/billing/plans", handler.GetPlans)

	req := httptest.NewRequest(http.MethodGet, "/billing/plans", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	data := result["data"].([]interface{})
	assert.Len(t, data, 3)
}

func TestHandler_GenerateCheckoutURL_InvalidBody(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/checkout-url", handler.GenerateCheckoutURL)

	req := httptest.NewRequest(http.MethodPost, "/billing/checkout-url", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_GenerateCheckoutURL_ValidationError(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/checkout-url", handler.GenerateCheckoutURL)

	requestBody := map[string]interface{}{
		"annual": true,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/billing/checkout-url", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestHandler_GenerateCheckoutURL_PlanNotFound(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/checkout-url", handler.GenerateCheckoutURL)

	requestBody := map[string]interface{}{
		"plan_id": "nonexistent",
		"annual":  false,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/billing/checkout-url", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_CancelSubscription_InvalidBody(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/cancel-subscription", handler.CancelSubscription)

	req := httptest.NewRequest(http.MethodPost, "/billing/cancel-subscription", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_CancelSubscription_ValidationError(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/cancel-subscription", handler.CancelSubscription)

	requestBody := map[string]interface{}{}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/billing/cancel-subscription", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestHandler_CancelSubscription_NotFound(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/cancel-subscription", handler.CancelSubscription)

	requestBody := map[string]interface{}{
		"subscription_id": "nonexistent",
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/billing/cancel-subscription", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_ResumeSubscription_InvalidBody(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/resume-subscription", handler.ResumeSubscription)

	req := httptest.NewRequest(http.MethodPost, "/billing/resume-subscription", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_ResumeSubscription_ValidationError(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/resume-subscription", handler.ResumeSubscription)

	requestBody := map[string]interface{}{}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/billing/resume-subscription", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestHandler_ResumeSubscription_NotFound(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Post("/billing/resume-subscription", handler.ResumeSubscription)

	requestBody := map[string]interface{}{
		"subscription_id": "nonexistent",
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/billing/resume-subscription", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_ResumeSubscription_NotCancelled(t *testing.T) {
	handler, app, repo, _ := setupTestHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Post("/billing/resume-subscription", handler.ResumeSubscription)

	requestBody := map[string]interface{}{
		"subscription_id": sub.ID,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/billing/resume-subscription", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_GetSubscriptions(t *testing.T) {
	handler, app, repo, _ := setupTestHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Get("/billing/subscriptions", handler.GetSubscriptions)

	req := httptest.NewRequest(http.MethodGet, "/billing/subscriptions", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_GetSubscription(t *testing.T) {
	handler, app, repo, _ := setupTestHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Get("/billing/subscriptions/:id", handler.GetSubscription)

	req := httptest.NewRequest(http.MethodGet, "/billing/subscriptions/"+sub.ID, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_GetSubscription_NotFound(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Get("/billing/subscriptions/:id", handler.GetSubscription)

	req := httptest.NewRequest(http.MethodGet, "/billing/subscriptions/nonexistent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_GetOrders(t *testing.T) {
	handler, app, repo, _ := setupTestHandler(t)

	order := &models.Order{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_order_1",
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

	app.Get("/billing/orders", handler.GetOrders)

	req := httptest.NewRequest(http.MethodGet, "/billing/orders", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_GetSubscriptionOptions(t *testing.T) {
	handler, app, repo, _ := setupTestHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "pro_monthly",
		VariantID:      "pro_monthly",
		Name:           "Pro",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Get("/billing/options", handler.GetSubscriptionOptions)

	req := httptest.NewRequest(http.MethodGet, "/billing/options", nil)
	resp, err := app.Test(req, -1) // disable timeout
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]interface{})
	assert.True(t, data["is_subscribed"].(bool))
	assert.Equal(t, float64(10), data["max_servers"])
}

func TestHandler_RegisterSubscription_AlreadySubscribed(t *testing.T) {
	handler, app, repo, _ := setupTestHandler(t)

	sub := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	app.Get("/register/subscription", handler.RegisterSubscription)

	req := httptest.NewRequest(http.MethodGet, "/register/subscription", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_RegisterSubscription_NoSubscription(t *testing.T) {
	handler, app, _, _ := setupTestHandler(t)

	app.Get("/register/subscription", handler.RegisterSubscription)

	req := httptest.NewRequest(http.MethodGet, "/register/subscription", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	data := result["data"].([]interface{})
	assert.Len(t, data, 3)
}

func TestHandler_Index_NoServerCountFn(t *testing.T) {
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
	handler := handlers.NewBillingHandler(service, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("teamID", "team_1")
		return c.Next()
	})

	app.Get("/billing", handler.Index)

	req := httptest.NewRequest(http.MethodGet, "/billing", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data := result["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["server_count"])
}

func TestHandler_GetSubscriptionOptions_WithRole(t *testing.T) {
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
	handler := handlers.NewBillingHandler(service, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("teamID", "team_1")
		c.Locals("userRole", "admin")
		return c.Next()
	})

	app.Get("/billing/options", handler.GetSubscriptionOptions)

	req := httptest.NewRequest(http.MethodGet, "/billing/options", nil)
	resp, err := app.Test(req, -1) // disable timeout
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data := result["data"].(map[string]interface{})
	assert.Equal(t, float64(999999), data["max_servers"])
}
