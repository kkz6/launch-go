package billing

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestService(t *testing.T) (*Service, *Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&Subscription{}, &Order{}, &WebhookEvent{})
	require.NoError(t, err)

	repo := NewRepository(db)
	log := zerolog.Nop()

	config := &Config{
		SubscriptionsEnabled: true,
		Plans:                DefaultPlans(),
	}

	service := NewService(repo, nil, config, &log)

	return service, repo, db
}

func TestService_GetPlans(t *testing.T) {
	service, _, _ := setupTestService(t)

	plans := service.GetPlans()
	assert.Len(t, plans, 3)
	assert.Equal(t, "Starter", plans[0].Name)
	assert.Equal(t, "Pro", plans[1].Name)
	assert.Equal(t, "Enterprise", plans[2].Name)
}

func TestService_GetPlanByID(t *testing.T) {
	service, _, _ := setupTestService(t)

	plan := service.GetPlanByID("1")
	assert.NotNil(t, plan)
	assert.Equal(t, "Starter", plan.Name)

	plan = service.GetPlanByID("nonexistent")
	assert.Nil(t, plan)
}

func TestService_GetPlanByProductID(t *testing.T) {
	service, _, _ := setupTestService(t)

	plan := service.GetPlanByProductID("starter_monthly")
	assert.NotNil(t, plan)
	assert.Equal(t, "Starter", plan.Name)

	plan = service.GetPlanByProductID("starter_yearly")
	assert.NotNil(t, plan)
	assert.Equal(t, "Starter", plan.Name)

	plan = service.GetPlanByProductID("nonexistent")
	assert.Nil(t, plan)
}

func TestService_GetPlanByVariantID(t *testing.T) {
	service, _, _ := setupTestService(t)

	plan := service.GetPlanByVariantID("pro_monthly")
	assert.NotNil(t, plan)
	assert.Equal(t, "Pro", plan.Name)

	plan = service.GetPlanByVariantID("pro_yearly")
	assert.NotNil(t, plan)
	assert.Equal(t, "Pro", plan.Name)
}

func TestService_GetSubscriptions(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub1 := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub1)

	sub2 := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_2",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusCancelled,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub2)

	subscriptions, err := service.GetSubscriptions(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Len(t, subscriptions, 2)
}

func TestService_GetActiveSubscription(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	found, err := service.GetActiveSubscription(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Equal(t, sub.ID, found.ID)
}

func TestService_GetSubscriptionByID(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	found, err := service.GetSubscriptionByID(context.Background(), sub.ID)
	assert.NoError(t, err)
	assert.Equal(t, sub.ID, found.ID)
}

func TestService_IsSubscribed(t *testing.T) {
	service, repo, _ := setupTestService(t)

	subscribed, err := service.IsSubscribed(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.False(t, subscribed)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	subscribed, err = service.IsSubscribed(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.True(t, subscribed)
}

func TestService_GetOrders(t *testing.T) {
	service, repo, _ := setupTestService(t)

	order := &Order{
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
		Status:         OrderStatusPaid,
	}
	repo.CreateOrder(context.Background(), order)

	orders, err := service.GetOrders(context.Background(), "team_1")
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
}

func TestService_GetBillingData(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	order := &Order{
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
		Status:         OrderStatusPaid,
	}
	repo.CreateOrder(context.Background(), order)

	data, err := service.GetBillingData(context.Background(), "team_1", 5)
	assert.NoError(t, err)
	assert.Equal(t, 5, data.ServerCount)
	assert.Len(t, data.Subscriptions, 1)
	assert.Len(t, data.Receipts, 1)
	assert.Len(t, data.SubscriptionPlans, 3)
}

func TestTeamSubscriptionOptions_MustVerifySubscription(t *testing.T) {
	service, _, _ := setupTestService(t)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.True(t, options.MustVerifySubscription())

	service.config.SubscriptionsEnabled = false
	assert.False(t, options.MustVerifySubscription())
}

func TestTeamSubscriptionOptions_IsAdmin(t *testing.T) {
	service, _, _ := setupTestService(t)

	customerOptions := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.False(t, customerOptions.IsAdmin())

	managerOptions := NewTeamSubscriptionOptions("team_1", service, UserRoleManager, nil, nil, nil)
	assert.True(t, managerOptions.IsAdmin())

	adminOptions := NewTeamSubscriptionOptions("team_1", service, UserRoleAdmin, nil, nil, nil)
	assert.True(t, adminOptions.IsAdmin())
}

func TestTeamSubscriptionOptions_IsSubscribed(t *testing.T) {
	service, repo, _ := setupTestService(t)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.False(t, options.IsSubscribed(context.Background()))

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	assert.True(t, options.IsSubscribed(context.Background()))
}

func TestTeamSubscriptionOptions_OnTrialOrIsSubscribed(t *testing.T) {
	service, repo, _ := setupTestService(t)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.False(t, options.OnTrialOrIsSubscribed(context.Background()))

	adminOptions := NewTeamSubscriptionOptions("team_1", service, UserRoleAdmin, nil, nil, nil)
	assert.True(t, adminOptions.OnTrialOrIsSubscribed(context.Background()))

	future := time.Now().Add(7 * 24 * time.Hour)
	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusOnTrial,
		TrialEndsAt:    &future,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	assert.True(t, options.OnTrialOrIsSubscribed(context.Background()))
}

func TestTeamSubscriptionOptions_OnTrialOrIsSubscribed_SubscriptionsDisabled(t *testing.T) {
	service, _, _ := setupTestService(t)
	service.config.SubscriptionsEnabled = false

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.True(t, options.OnTrialOrIsSubscribed(context.Background()))
}

func TestTeamSubscriptionOptions_CanCreateServer(t *testing.T) {
	service, repo, _ := setupTestService(t)

	serverCount := 0
	serverCountFn := func(ctx context.Context, teamID string) (int, error) {
		return serverCount, nil
	}

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, serverCountFn, nil, nil)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	assert.True(t, options.CanCreateServer(context.Background()))

	serverCount = 3
	options = NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, serverCountFn, nil, nil)
	assert.False(t, options.CanCreateServer(context.Background()))

	adminOptions := NewTeamSubscriptionOptions("team_1", service, UserRoleAdmin, serverCountFn, nil, nil)
	assert.True(t, adminOptions.CanCreateServer(context.Background()))
}

func TestTeamSubscriptionOptions_CanCreateServer_SubscriptionsDisabled(t *testing.T) {
	service, _, _ := setupTestService(t)
	service.config.SubscriptionsEnabled = false

	serverCountFn := func(ctx context.Context, teamID string) (int, error) {
		return 100, nil
	}

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, serverCountFn, nil, nil)
	assert.True(t, options.CanCreateServer(context.Background()))
}

func TestTeamSubscriptionOptions_CanCreateSiteOnServer(t *testing.T) {
	service, repo, _ := setupTestService(t)

	siteCount := 0
	siteCountFn := func(ctx context.Context, serverID string) (int, error) {
		return siteCount, nil
	}

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, siteCountFn)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	assert.True(t, options.CanCreateSiteOnServer(context.Background(), "server_1"))

	siteCount = 5
	options = NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, siteCountFn)
	assert.False(t, options.CanCreateSiteOnServer(context.Background(), "server_1"))
}

func TestTeamSubscriptionOptions_CanAddTeamMember(t *testing.T) {
	service, repo, _ := setupTestService(t)

	memberCount := 0
	memberCountFn := func(ctx context.Context, teamID string) (int, error) {
		return memberCount, nil
	}

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, memberCountFn, nil)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	assert.True(t, options.CanAddTeamMember(context.Background()))

	memberCount = 1
	options = NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, memberCountFn, nil)
	assert.False(t, options.CanAddTeamMember(context.Background()))
}

func TestTeamSubscriptionOptions_MaxServers(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.Equal(t, 3, options.MaxServers(context.Background()))

	adminOptions := NewTeamSubscriptionOptions("team_1", service, UserRoleAdmin, nil, nil, nil)
	assert.Equal(t, 999999, adminOptions.MaxServers(context.Background()))
}

func TestTeamSubscriptionOptions_MaxSitesPerServer(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "pro_monthly",
		VariantID:      "pro_monthly",
		Name:           "Pro",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.Equal(t, 20, options.MaxSitesPerServer(context.Background()))
}

func TestTeamSubscriptionOptions_MaxDeploymentsPerSite(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "pro_monthly",
		VariantID:      "pro_monthly",
		Name:           "Pro",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.Equal(t, 10, options.MaxDeploymentsPerSite(context.Background()))

	service.config.SubscriptionsEnabled = false
	options = NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.Equal(t, 0, options.MaxDeploymentsPerSite(context.Background()))
}

func TestTeamSubscriptionOptions_HasBackups(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "enterprise_monthly",
		VariantID:      "enterprise_monthly",
		Name:           "Enterprise",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.True(t, options.HasBackups(context.Background()))

	sub.ProductID = "starter_monthly"
	sub.VariantID = "starter_monthly"
	repo.UpdateSubscription(context.Background(), sub)

	options = NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.False(t, options.HasBackups(context.Background()))
}

func TestTeamSubscriptionOptions_HasMonitoring(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "pro_monthly",
		VariantID:      "pro_monthly",
		Name:           "Pro",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.True(t, options.HasMonitoring(context.Background()))

	sub.ProductID = "starter_monthly"
	sub.VariantID = "starter_monthly"
	repo.UpdateSubscription(context.Background(), sub)

	options = NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	assert.False(t, options.HasMonitoring(context.Background()))
}

func TestTeamSubscriptionOptions_PlanOptions_FreeLimits(t *testing.T) {
	service, _, _ := setupTestService(t)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	planOptions := options.PlanOptions(context.Background())

	assert.Equal(t, FreeLimits.MaxServers, planOptions.MaxServers)
	assert.Equal(t, FreeLimits.MaxSitesPerServer, planOptions.MaxSitesPerServer)
	assert.Equal(t, FreeLimits.HasBackups, planOptions.HasBackups)
}

func TestTeamSubscriptionOptions_PlanOptions_AdminLimits(t *testing.T) {
	service, _, _ := setupTestService(t)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleAdmin, nil, nil, nil)
	planOptions := options.PlanOptions(context.Background())

	assert.Equal(t, AdminLimits.MaxServers, planOptions.MaxServers)
	assert.Equal(t, AdminLimits.MaxSitesPerServer, planOptions.MaxSitesPerServer)
	assert.Equal(t, AdminLimits.HasBackups, planOptions.HasBackups)
}

func TestTeamSubscriptionOptions_PlanOptions_TrialLimits(t *testing.T) {
	service, repo, _ := setupTestService(t)

	future := time.Now().Add(7 * 24 * time.Hour)
	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusOnTrial,
		TrialEndsAt:    &future,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)
	planOptions := options.PlanOptions(context.Background())

	assert.Equal(t, 3, planOptions.MaxServers)
}

func TestTeamSubscriptionOptions_GetSubscriptionOptionsResponse(t *testing.T) {
	service, repo, _ := setupTestService(t)

	serverCountFn := func(ctx context.Context, teamID string) (int, error) {
		return 2, nil
	}

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "pro_monthly",
		VariantID:      "pro_monthly",
		Name:           "Pro",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, serverCountFn, nil, nil)
	resp, err := options.GetSubscriptionOptionsResponse(context.Background())

	assert.NoError(t, err)
	assert.True(t, resp.IsSubscribed)
	assert.False(t, resp.OnTrial)
	assert.Equal(t, 10, resp.MaxServers)
	assert.Equal(t, 20, resp.MaxSitesPerServer)
	assert.True(t, resp.HasMonitoring)
	assert.True(t, resp.CanCreateServer)
	assert.Equal(t, 2, resp.ServerCount)
}

func TestTeamSubscriptionOptions_CountServers(t *testing.T) {
	service, _, _ := setupTestService(t)

	count, err := (&TeamSubscriptionOptions{service: service}).CountServers(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	serverCountFn := func(ctx context.Context, teamID string) (int, error) {
		return 5, nil
	}

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, serverCountFn, nil, nil)
	count, err = options.CountServers(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestTeamSubscriptionOptions_CountSitesOnServer(t *testing.T) {
	service, _, _ := setupTestService(t)

	count, err := (&TeamSubscriptionOptions{service: service}).CountSitesOnServer(context.Background(), "server_1")
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	siteCountFn := func(ctx context.Context, serverID string) (int, error) {
		return 10, nil
	}

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, siteCountFn)
	count, err = options.CountSitesOnServer(context.Background(), "server_1")
	assert.NoError(t, err)
	assert.Equal(t, 10, count)
}

func TestTeamSubscriptionOptions_CountTeamMembers(t *testing.T) {
	service, _, _ := setupTestService(t)

	count, err := (&TeamSubscriptionOptions{service: service}).CountTeamMembers(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	memberCountFn := func(ctx context.Context, teamID string) (int, error) {
		return 3, nil
	}

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, memberCountFn, nil)
	count, err = options.CountTeamMembers(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestTeamSubscriptionOptions_CachesValues(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "starter_monthly",
		VariantID:      "starter_monthly",
		Name:           "Starter",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	options := NewTeamSubscriptionOptions("team_1", service, UserRoleCustomer, nil, nil, nil)

	planOptions1 := options.PlanOptions(context.Background())
	planOptions2 := options.PlanOptions(context.Background())

	assert.Equal(t, planOptions1.MaxServers, planOptions2.MaxServers)

	isAdmin1 := options.IsAdmin()
	isAdmin2 := options.IsAdmin()

	assert.Equal(t, isAdmin1, isAdmin2)
}

func TestService_GenerateCheckoutURL_PlanNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)

	req := &GenerateCheckoutURLRequest{
		PlanID: "nonexistent",
		Annual: false,
	}

	_, err := service.GenerateCheckoutURL(context.Background(), "team_1", req, "http://example.com")
	assert.ErrorIs(t, err, ErrPlanNotFound)
}

func TestService_CancelSubscription_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)

	err := service.CancelSubscription(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrSubscriptionNotFound)
}

func TestService_ResumeSubscription_NotCancelled(t *testing.T) {
	service, repo, _ := setupTestService(t)

	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusActive,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	err := service.ResumeSubscription(context.Background(), sub.ID)
	assert.ErrorIs(t, err, ErrSubscriptionNotCancelled)
}

func TestService_ResumeSubscription_GracePeriodEnded(t *testing.T) {
	service, repo, _ := setupTestService(t)

	past := time.Now().Add(-24 * time.Hour)
	sub := &Subscription{
		TeamID:         "team_1",
		LemonSqueezyID: "ls_1",
		ProductID:      "prod_1",
		VariantID:      "var_1",
		Name:           "Test",
		Status:         SubscriptionStatusCancelled,
		EndsAt:         &past,
		BillingAnchor:  1,
	}
	repo.CreateSubscription(context.Background(), sub)

	err := service.ResumeSubscription(context.Background(), sub.ID)
	assert.ErrorIs(t, err, ErrCannotResume)
}
