package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
)

const (
	productMonthlyA = "prod_monthly_a" // 699 cents/mo
	productYearlyB  = "prod_yearly_b"  // 8388 cents/yr -> 699/mo
)

func setupOverviewDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Subscription and Order both declare a composite index named idx_billable.
	// sqlite index names are database-global (MySQL scopes them per-table), so
	// migrating both in one DB trips a duplicate-index error on the second
	// table. Migrate each model independently and tolerate that specific
	// collision — the table itself is still created.
	for _, model := range []any{
		&billingmodels.Subscription{},
		&billingmodels.Order{},
		&authmodels.Team{},
	} {
		err := db.AutoMigrate(model)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			require.NoError(t, err)
		}
	}

	return db
}

func ptrTime(tm time.Time) *time.Time { return &tm }

func TestOverview(t *testing.T) {
	db := setupOverviewDB(t)
	repo := repositories.NewRegistry(db)
	svc := NewService(repo)

	// Inject a deterministic product -> monthly-equivalent-cents map so the test
	// does not depend on plan config. Yearly maps to its monthly equivalent.
	svc.SetMonthlyEquivByProduct(map[string]int64{
		productMonthlyA: 699,
		productYearlyB:  8388 / 12, // 699
	})

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	startThisMonth := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	startLastMonth := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	twoMonthsAgo := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	team := authmodels.Team{Name: "Acme Inc"}
	team.ID = "team_acme00000000000000000"
	require.NoError(t, db.Create(&team).Error)

	team2 := authmodels.Team{Name: "Globex"}
	team2.ID = "team_globex0000000000000000"
	require.NoError(t, db.Create(&team2).Error)

	// Subscriptions: 2 active (monthly + yearly plan) + 1 on_trial.
	subs := []billingmodels.Subscription{
		{
			BillableType:           billingmodels.BillableTypeTeam,
			BillableID:             team.ID,
			Type:                   "default",
			ProviderSubscriptionID: "sub_active_monthly",
			Status:                 billingtypes.SubscriptionStatusActive,
			ProductID:              productMonthlyA,
			CreatedAt:              startThisMonth.Add(2 * time.Hour), // MTD
		},
		{
			BillableType:           billingmodels.BillableTypeTeamLegacy,
			BillableID:             team2.ID,
			Type:                   "default",
			ProviderSubscriptionID: "sub_active_yearly",
			Status:                 billingtypes.SubscriptionStatusActive,
			ProductID:              productYearlyB,
			CreatedAt:              twoMonthsAgo, // not MTD
		},
		{
			BillableType:           billingmodels.BillableTypeTeam,
			BillableID:             team.ID,
			Type:                   "default",
			ProviderSubscriptionID: "sub_trial",
			Status:                 billingtypes.SubscriptionStatusOnTrial,
			ProductID:              productMonthlyA,
			TrialEndsAt:            ptrTime(now.Add(7 * 24 * time.Hour)),
			CreatedAt:              startThisMonth.Add(3 * time.Hour), // MTD
		},
		// A cancelled sub updated this month -> CancelledMTD = 1.
		{
			BillableType:           billingmodels.BillableTypeTeam,
			BillableID:             team.ID,
			Type:                   "default",
			ProviderSubscriptionID: "sub_cancelled",
			Status:                 billingtypes.SubscriptionStatusCancelled,
			ProductID:              productMonthlyA,
			CreatedAt:              twoMonthsAgo,
			UpdatedAt:              startThisMonth.Add(time.Hour),
		},
	}
	require.NoError(t, db.Create(&subs).Error)

	// Orders.
	orders := []billingmodels.Order{
		// This month, paid: 699 + 2000 = 2699
		newOrder(team.ID, "ord_tm_1", 699, "USD", billingtypes.OrderStatusPaid, now.Add(-2*time.Hour)),
		newOrder(team2.ID, "ord_tm_2", 2000, "USD", billingtypes.OrderStatusPaid, startThisMonth.Add(5*time.Hour)),
		// Last month, paid: 8388
		newOrder(team2.ID, "ord_lm_1", 8388, "USD", billingtypes.OrderStatusPaid, startLastMonth.Add(2*24*time.Hour)),
		// Two months ago, paid: 500
		newOrder(team.ID, "ord_old_1", 500, "USD", billingtypes.OrderStatusPaid, twoMonthsAgo),
		// Not paid - excluded everywhere.
		newOrder(team.ID, "ord_pending", 9999, "USD", billingtypes.OrderStatusPending, now.Add(-time.Hour)),
		newOrder(team.ID, "ord_refunded", 1234, "USD", billingtypes.OrderStatusRefunded, now.Add(-time.Hour)),
	}
	require.NoError(t, db.Create(&orders).Error)

	ov, err := svc.Overview(context.Background(), now)
	require.NoError(t, err)
	require.NotNil(t, ov)

	// MRR = monthly(699) + yearly-monthly-equiv(699) = 1398. Trial excluded? No:
	// spec says MRR loads active + on_trial. Trial sub product is monthlyA (699).
	assert.Equal(t, int64(699+699+699), ov.MRRCents)

	// Active = status active only; trial = on_trial with future trial end.
	assert.Equal(t, int64(2), ov.ActiveSubscriptions)
	assert.Equal(t, int64(1), ov.TrialSubscriptions)

	// New subs MTD: 2 created this month (active monthly + trial).
	assert.Equal(t, int64(2), ov.NewSubscriptionsMTD)

	// Cancelled MTD: 1.
	assert.Equal(t, int64(1), ov.CancelledMTD)

	// Total revenue = sum of all paid orders = 699+2000+8388+500 = 11587.
	assert.Equal(t, int64(699+2000+8388+500), ov.TotalRevenueCents)

	// This month revenue = 699 + 2000 = 2699.
	assert.Equal(t, int64(2699), ov.RevenueThisMonthCents)

	// Last month revenue = 8388.
	assert.Equal(t, int64(8388), ov.RevenueLastMonthCents)

	// Currency = dominant among paid orders = USD.
	assert.Equal(t, "USD", ov.Currency)

	// Recent payments: latest paid orders, DESC by ordered_at; team name resolved.
	require.NotEmpty(t, ov.RecentPayments)
	assert.Equal(t, 4, len(ov.RecentPayments)) // only paid orders
	// Most recent paid order is ord_tm_1 (now-2h).
	assert.Equal(t, int64(699), ov.RecentPayments[0].Total)
	assert.Equal(t, "Acme Inc", ov.RecentPayments[0].TeamName)
	assert.Equal(t, team.ID, ov.RecentPayments[0].TeamID)
	// Ordering is descending by ordered_at.
	for i := 1; i < len(ov.RecentPayments); i++ {
		prev := ov.RecentPayments[i-1].OrderedAt
		cur := ov.RecentPayments[i].OrderedAt
		require.NotNil(t, prev)
		require.NotNil(t, cur)
		assert.False(t, cur.After(*prev), "recent payments must be ordered_at DESC")
	}

	// Revenue trend: last 6 months ascending, ending with current month.
	require.Len(t, ov.RevenueTrend, 6)
	assert.Equal(t, "2026-01", ov.RevenueTrend[0].Month)
	assert.Equal(t, "2026-06", ov.RevenueTrend[5].Month)
	assert.Equal(t, int64(2699), ov.RevenueTrend[5].Total) // June
	assert.Equal(t, int64(8388), ov.RevenueTrend[4].Total) // May
	assert.Equal(t, int64(500), ov.RevenueTrend[3].Total)  // April
	assert.Equal(t, int64(0), ov.RevenueTrend[2].Total)    // March
}

func TestOverview_EmptyDefaultsToUSD(t *testing.T) {
	db := setupOverviewDB(t)
	svc := NewService(repositories.NewRegistry(db))
	svc.SetMonthlyEquivByProduct(map[string]int64{})

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	ov, err := svc.Overview(context.Background(), now)
	require.NoError(t, err)
	assert.Equal(t, "USD", ov.Currency)
	assert.Equal(t, int64(0), ov.MRRCents)
	assert.Equal(t, int64(0), ov.TotalRevenueCents)
	assert.Empty(t, ov.RecentPayments)
	assert.Len(t, ov.RevenueTrend, 6)
}

func newOrder(teamID, providerID string, total int64, currency string, status billingtypes.OrderStatus, orderedAt time.Time) billingmodels.Order {
	return billingmodels.Order{
		BillableType:    billingmodels.BillableTypeTeam,
		BillableID:      teamID,
		Provider:        billingmodels.ProviderDodoPayments,
		ProviderOrderID: providerID,
		CustomerID:      "cus_test",
		Identifier:      providerID,
		ProductID:       "prod_x",
		VariantID:       "var_x",
		Currency:        currency,
		Total:           total,
		Status:          status,
		OrderedAt:       orderedAt,
	}
}
