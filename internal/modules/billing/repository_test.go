package billing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Subscription{}, &models.Order{}, &models.WebhookEvent{})
	require.NoError(t, err)

	return db
}

var subscriptionCounter int

func createTestSubscription(t *testing.T, repo *repositories.BillingRepository, teamID string, status enums.SubscriptionStatus) *models.Subscription {
	subscriptionCounter++
	subscription := &models.Subscription{
		TeamID:         teamID,
		LemonSqueezyID: fmt.Sprintf("ls_%s_%d", teamID, subscriptionCounter),
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test Plan",
		Status:         status,
		BillingAnchor:  1,
	}

	err := repo.CreateSubscription(context.Background(), subscription)
	require.NoError(t, err)
	require.NotEmpty(t, subscription.ID)

	return subscription
}

var orderCounter int

func createTestOrder(t *testing.T, repo *repositories.BillingRepository, teamID string) *models.Order {
	orderCounter++
	order := &models.Order{
		TeamID:         teamID,
		LemonSqueezyID: fmt.Sprintf("ls_order_%s_%d", teamID, orderCounter),
		CustomerID:     "cust_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		OrderNumber:    fmt.Sprintf("ORD_%d", orderCounter),
		Currency:       "USD",
		CurrencyRate:   "1.0",
		Subtotal:       1000,
		DiscountTotal:  0,
		Tax:            100,
		Total:          1100,
		Status:         enums.OrderStatusPaid,
	}

	err := repo.CreateOrder(context.Background(), order)
	require.NoError(t, err)
	require.NotEmpty(t, order.ID)

	return order
}

func TestRepository_CreateSubscription(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	subscription := &models.Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_123",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Pro Plan",
		Status:         enums.SubscriptionStatusActive,
		BillingAnchor:  1,
	}

	err := repo.CreateSubscription(context.Background(), subscription)
	assert.NoError(t, err)
	assert.NotEmpty(t, subscription.ID)
}

func TestRepository_FindSubscriptionByID(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	created := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	found, err := repo.FindSubscriptionByID(context.Background(), created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.TeamID, found.TeamID)
	assert.Equal(t, created.Status, found.Status)
}

func TestRepository_FindSubscriptionByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	_, err := repo.FindSubscriptionByID(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestRepository_FindSubscriptionByLemonSqueezyID(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	created := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	found, err := repo.FindSubscriptionByLemonSqueezyID(context.Background(), created.LemonSqueezyID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestRepository_FindSubscriptionsByTeam(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)
	createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusCancelled)
	createTestSubscription(t, repo, "team_2", enums.SubscriptionStatusActive)

	subscriptions, err := repo.FindSubscriptionsByTeam(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Len(t, subscriptions, 2)
}

func TestRepository_FindActiveSubscriptionByTeam(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusCancelled)
	active := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	found, err := repo.FindActiveSubscriptionByTeam(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Equal(t, active.ID, found.ID)
}

func TestRepository_FindActiveSubscriptionByTeam_OnTrial(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	trial := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusOnTrial)

	found, err := repo.FindActiveSubscriptionByTeam(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Equal(t, trial.ID, found.ID)
}

func TestRepository_FindActiveSubscriptionByTeam_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusCancelled)

	_, err := repo.FindActiveSubscriptionByTeam(context.Background(), "team_1")
	assert.Error(t, err)
}

func TestRepository_UpdateSubscription(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	subscription := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	subscription.Status = enums.SubscriptionStatusCancelled
	now := time.Now()
	subscription.EndsAt = &now

	err := repo.UpdateSubscription(context.Background(), subscription)
	assert.NoError(t, err)

	found, err := repo.FindSubscriptionByID(context.Background(), subscription.ID)
	assert.NoError(t, err)
	assert.Equal(t, enums.SubscriptionStatusCancelled, found.Status)
	assert.NotNil(t, found.EndsAt)
}

func TestRepository_UpdateSubscriptionStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	subscription := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	err := repo.UpdateSubscriptionStatus(context.Background(), subscription.ID, enums.SubscriptionStatusPaused)
	assert.NoError(t, err)

	found, err := repo.FindSubscriptionByID(context.Background(), subscription.ID)
	assert.NoError(t, err)
	assert.Equal(t, enums.SubscriptionStatusPaused, found.Status)
}

func TestRepository_UpdateSubscriptionFields(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	subscription := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	cardBrand := "visa"
	err := repo.UpdateSubscriptionFields(context.Background(), subscription.ID, map[string]interface{}{
		"card_brand":     cardBrand,
		"card_last_four": "4242",
	})
	assert.NoError(t, err)

	found, err := repo.FindSubscriptionByID(context.Background(), subscription.ID)
	assert.NoError(t, err)
	assert.Equal(t, "visa", *found.CardBrand)
	assert.Equal(t, "4242", *found.CardLastFour)
}

func TestRepository_DeleteSubscription(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	subscription := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	err := repo.DeleteSubscription(context.Background(), subscription.ID)
	assert.NoError(t, err)

	_, err = repo.FindSubscriptionByID(context.Background(), subscription.ID)
	assert.Error(t, err)
}

func TestRepository_CountActiveSubscriptionsByTeam(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)
	createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusOnTrial)
	createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusCancelled)

	count, err := repo.CountActiveSubscriptionsByTeam(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestRepository_IsTeamSubscribed(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	subscribed, err := repo.IsTeamSubscribed(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.True(t, subscribed)

	subscribed, err = repo.IsTeamSubscribed(context.Background(), "team_2")
	assert.NoError(t, err)
	assert.False(t, subscribed)
}

func TestRepository_CreateOrder(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

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
		Total:          1100,
		Status:         enums.OrderStatusPaid,
	}

	err := repo.CreateOrder(context.Background(), order)
	assert.NoError(t, err)
	assert.NotEmpty(t, order.ID)
}

func TestRepository_FindOrderByID(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	created := createTestOrder(t, repo, "team_1")

	found, err := repo.FindOrderByID(context.Background(), created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.OrderNumber, found.OrderNumber)
}

func TestRepository_FindOrderByLemonSqueezyID(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	created := createTestOrder(t, repo, "team_1")

	found, err := repo.FindOrderByLemonSqueezyID(context.Background(), created.LemonSqueezyID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestRepository_FindOrdersByTeam(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	createTestOrder(t, repo, "team_1")
	createTestOrder(t, repo, "team_1")
	createTestOrder(t, repo, "team_2")

	orders, err := repo.FindOrdersByTeam(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Len(t, orders, 2)
}

func TestRepository_FindOrdersBySubscription(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	subscription := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	order := &models.Order{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_order_1",
		SubscriptionID: &subscription.ID,
		CustomerID:     "cust_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		OrderNumber:    "123",
		Currency:       "USD",
		CurrencyRate:   "1.0",
		Subtotal:       1000,
		Total:          1100,
		Status:         enums.OrderStatusPaid,
	}

	err := repo.CreateOrder(context.Background(), order)
	require.NoError(t, err)

	orders, err := repo.FindOrdersBySubscription(context.Background(), subscription.ID)
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
}

func TestRepository_UpdateOrder(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	order := createTestOrder(t, repo, "team_1")

	order.Status = enums.OrderStatusRefunded
	now := time.Now()
	order.RefundedAt = &now

	err := repo.UpdateOrder(context.Background(), order)
	assert.NoError(t, err)

	found, err := repo.FindOrderByID(context.Background(), order.ID)
	assert.NoError(t, err)
	assert.Equal(t, enums.OrderStatusRefunded, found.Status)
}

func TestRepository_UpdateOrderStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	order := createTestOrder(t, repo, "team_1")

	err := repo.UpdateOrderStatus(context.Background(), order.ID, enums.OrderStatusRefunded)
	assert.NoError(t, err)

	found, err := repo.FindOrderByID(context.Background(), order.ID)
	assert.NoError(t, err)
	assert.Equal(t, enums.OrderStatusRefunded, found.Status)
}

func TestRepository_CreateWebhookEvent(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	event := &models.WebhookEvent{
		EventName: enums.WebhookEventSubscriptionCreated,
		Payload:   `{"data": "test"}`,
		Signature: "sig_123",
		Processed: false,
	}

	err := repo.CreateWebhookEvent(context.Background(), event)
	assert.NoError(t, err)
	assert.NotEmpty(t, event.ID)
}

func TestRepository_FindWebhookEventByID(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	event := &models.WebhookEvent{
		EventName: enums.WebhookEventSubscriptionCreated,
		Payload:   `{"data": "test"}`,
		Signature: "sig_123",
		Processed: false,
	}

	err := repo.CreateWebhookEvent(context.Background(), event)
	require.NoError(t, err)

	found, err := repo.FindWebhookEventByID(context.Background(), event.ID)
	assert.NoError(t, err)
	assert.Equal(t, event.ID, found.ID)
	assert.Equal(t, event.EventName, found.EventName)
}

func TestRepository_FindUnprocessedWebhookEvents(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	event1 := &models.WebhookEvent{
		EventName:  enums.WebhookEventSubscriptionCreated,
		Payload:    `{}`,
		Signature:  "sig_1",
		Processed:  false,
		RetryCount: 0,
	}
	event2 := &models.WebhookEvent{
		EventName:  enums.WebhookEventSubscriptionUpdated,
		Payload:    `{}`,
		Signature:  "sig_2",
		Processed:  true,
		RetryCount: 0,
	}
	event3 := &models.WebhookEvent{
		EventName:  enums.WebhookEventOrderCreated,
		Payload:    `{}`,
		Signature:  "sig_3",
		Processed:  false,
		RetryCount: 5,
	}

	repo.CreateWebhookEvent(context.Background(), event1)
	repo.CreateWebhookEvent(context.Background(), event2)
	repo.CreateWebhookEvent(context.Background(), event3)

	events, err := repo.FindUnprocessedWebhookEvents(context.Background(), 3)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, event1.ID, events[0].ID)
}

func TestRepository_MarkWebhookEventProcessed(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	event := &models.WebhookEvent{
		EventName: enums.WebhookEventSubscriptionCreated,
		Payload:   `{}`,
		Signature: "sig_1",
		Processed: false,
	}

	repo.CreateWebhookEvent(context.Background(), event)

	err := repo.MarkWebhookEventProcessed(context.Background(), event.ID)
	assert.NoError(t, err)

	found, err := repo.FindWebhookEventByID(context.Background(), event.ID)
	assert.NoError(t, err)
	assert.True(t, found.Processed)
	assert.NotNil(t, found.ProcessedAt)
}

func TestRepository_MarkWebhookEventFailed(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	event := &models.WebhookEvent{
		EventName:  enums.WebhookEventSubscriptionCreated,
		Payload:    `{}`,
		Signature:  "sig_1",
		Processed:  false,
		RetryCount: 0,
	}

	repo.CreateWebhookEvent(context.Background(), event)

	err := repo.MarkWebhookEventFailed(context.Background(), event.ID, "test error")
	assert.NoError(t, err)

	found, err := repo.FindWebhookEventByID(context.Background(), event.ID)
	assert.NoError(t, err)
	assert.Equal(t, "test error", *found.Error)
	assert.Equal(t, 1, found.RetryCount)
}

func TestRepository_DeleteOldProcessedWebhookEvents(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)

	event1 := &models.WebhookEvent{
		EventName: enums.WebhookEventSubscriptionCreated,
		Payload:   `{}`,
		Signature: "sig_1",
		Processed: true,
	}

	repo.CreateWebhookEvent(context.Background(), event1)
	db.Model(&models.WebhookEvent{}).Where("id = ?", event1.ID).Update("processed_at", oldTime)

	event2 := &models.WebhookEvent{
		EventName: enums.WebhookEventSubscriptionUpdated,
		Payload:   `{}`,
		Signature: "sig_2",
		Processed: true,
	}

	repo.CreateWebhookEvent(context.Background(), event2)
	repo.MarkWebhookEventProcessed(context.Background(), event2.ID)

	err := repo.DeleteOldProcessedWebhookEvents(context.Background(), 24*time.Hour)
	assert.NoError(t, err)

	_, err = repo.FindWebhookEventByID(context.Background(), event1.ID)
	assert.Error(t, err)

	found, err := repo.FindWebhookEventByID(context.Background(), event2.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
}

func TestRepository_GetTeamSubscriptionInfo(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	subscription := createTestSubscription(t, repo, "team_1", enums.SubscriptionStatusActive)

	info, err := repo.GetTeamSubscriptionInfo(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, subscription.ID, info.SubscriptionID)
	assert.Equal(t, enums.SubscriptionStatusActive, info.Status)
}

func TestRepository_GetTeamSubscriptionInfo_NoSubscription(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	info, err := repo.GetTeamSubscriptionInfo(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Nil(t, info)
}

func TestRepository_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	err := repo.WithTransaction(context.Background(), func(tx *repositories.BillingRepository) error {
		subscription := &models.Subscription{
			TeamID:         "team_tx",
			LemonSqueezyID: "ls_tx_123",
			ProductID:      "prod_1",
			VariantID:      "var_1",
			Name:           "TX Plan",
			Status:         enums.SubscriptionStatusActive,
			BillingAnchor:  1,
		}

		if err := tx.CreateSubscription(context.Background(), subscription); err != nil {
			return err
		}

		order := &models.Order{
			TeamID:         "team_tx",
			LemonSqueezyID: "ls_order_tx",
			SubscriptionID: &subscription.ID,
			CustomerID:     "cust_1",
			ProductID:      "prod_1",
			VariantID:      "var_1",
			OrderNumber:    "TX123",
			Currency:       "USD",
			CurrencyRate:   "1.0",
			Subtotal:       1000,
			Total:          1000,
			Status:         enums.OrderStatusPaid,
		}

		return tx.CreateOrder(context.Background(), order)
	})

	assert.NoError(t, err)

	subscriptions, err := repo.FindSubscriptionsByTeam(context.Background(), "team_tx")
	assert.NoError(t, err)
	assert.Len(t, subscriptions, 1)

	orders, err := repo.FindOrdersByTeam(context.Background(), "team_tx")
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
}

func TestRepository_GetDB(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewBillingRepository(db)

	gotDB := repo.GetDB()
	assert.NotNil(t, gotDB)
	assert.Equal(t, db, gotDB)
}
