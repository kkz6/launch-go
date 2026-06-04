package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	staffmodels "github.com/kkz6/launch-go/internal/modules/staff/models"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// setupImpersonationDB returns an in-memory sqlite DB with just the
// impersonation_sessions table. The model has only char/timestamp/text columns,
// so it migrates cleanly under sqlite.
func setupImpersonationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&staffmodels.ImpersonationSession{}))
	return db
}

func TestRegistry_ImpersonationSessionLifecycle(t *testing.T) {
	db := setupImpersonationDB(t)
	repo := NewRegistry(db)
	ctx := context.Background()

	session := &staffmodels.ImpersonationSession{
		StaffID:      "staff-1",
		TargetUserID: "user-1",
	}

	require.NoError(t, repo.CreateImpersonationSession(ctx, session))
	require.NotEmpty(t, session.ID)
	require.False(t, session.StartedAt.IsZero())
	require.Nil(t, session.EndedAt)

	active, err := repo.ActiveImpersonationForStaff(ctx, "staff-1")
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, session.ID, active.ID)
	require.Equal(t, "user-1", active.TargetUserID)
	require.Nil(t, active.EndedAt)

	endedAt := time.Now()
	require.NoError(t, repo.EndImpersonationSession(ctx, session.ID, endedAt))

	ended, err := repo.ActiveImpersonationForStaff(ctx, "staff-1")
	require.NoError(t, err)
	require.Nil(t, ended)

	var stored staffmodels.ImpersonationSession
	require.NoError(t, db.First(&stored, "id = ?", session.ID).Error)
	require.NotNil(t, stored.EndedAt)
}

func TestRegistry_ActiveImpersonationForStaff_NoneReturnsNil(t *testing.T) {
	db := setupImpersonationDB(t)
	repo := NewRegistry(db)

	active, err := repo.ActiveImpersonationForStaff(context.Background(), "nobody")
	require.NoError(t, err)
	require.Nil(t, active)
}

// setupBillingDB returns an in-memory sqlite DB with the users, teams and
// subscriptions tables. All three are simple-column models that AutoMigrate
// cleanly under sqlite.
func setupBillingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&authmodels.User{},
		&authmodels.Team{},
		&billingmodels.Subscription{},
	))
	return db
}

func TestRegistry_ListUsersWithBilling_FoldsTeamsAndSubscriptions(t *testing.T) {
	db := setupBillingDB(t)
	repo := NewRegistry(db)
	ctx := context.Background()

	now := time.Now()
	older := now.Add(-time.Hour)
	trialEndsAt := now.Add(7 * 24 * time.Hour)

	userA := authmodels.User{
		BaseModel: baseModelAt(older),
		Name:      "Alice",
		Email:     "alice@example.com",
		Status:    authtypes.UserStatusActive,
	}
	userB := authmodels.User{
		BaseModel: baseModelAt(now),
		Name:      "Bob",
		Email:     "bob@example.com",
		Status:    authtypes.UserStatusActive,
	}
	require.NoError(t, db.Create(&userA).Error)
	require.NoError(t, db.Create(&userB).Error)

	// User A owns a personal team (with an active subscription) and a second
	// team with no subscription.
	teamAPersonal := authmodels.Team{Name: "Alice Personal", UserID: userA.ID, PersonalTeam: true}
	teamANoSub := authmodels.Team{Name: "Alice Side Project", UserID: userA.ID}
	require.NoError(t, db.Create(&teamAPersonal).Error)
	require.NoError(t, db.Create(&teamANoSub).Error)

	// User B owns one team with an on-trial subscription.
	teamB := authmodels.Team{Name: "Bob Team", UserID: userB.ID, PersonalTeam: true}
	require.NoError(t, db.Create(&teamB).Error)

	require.NoError(t, db.Create(&billingmodels.Subscription{
		BillableType:           billingmodels.BillableTypeTeam,
		BillableID:             teamAPersonal.ID,
		Type:                   "default",
		ProviderSubscriptionID: "sub_a_active",
		Status:                 billingtypes.SubscriptionStatusActive,
		ProductID:              "prod_1",
		VariantID:              "var_1",
	}).Error)
	require.NoError(t, db.Create(&billingmodels.Subscription{
		BillableType:           billingmodels.BillableTypeTeam,
		BillableID:             teamB.ID,
		Type:                   "default",
		ProviderSubscriptionID: "sub_b_trial",
		Status:                 billingtypes.SubscriptionStatusOnTrial,
		ProductID:              "prod_1",
		VariantID:              "var_1",
		TrialEndsAt:            &trialEndsAt,
	}).Error)

	users, teamsByOwner, subsByTeam, total, err := repo.ListUsersWithBilling(ctx, ListUsersOptions{Limit: 25, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, users, 2)

	// Ordered created_at DESC -> Bob (now) first, Alice (older) second.
	require.Equal(t, userB.ID, users[0].ID)
	require.Equal(t, userA.ID, users[1].ID)

	// User A owns two teams; only the personal one has a subscription.
	require.Len(t, teamsByOwner[userA.ID], 2)
	personalSub := subsByTeam[teamAPersonal.ID]
	require.NotNil(t, personalSub)
	require.Equal(t, billingtypes.SubscriptionStatusActive, personalSub.Status)
	require.Nil(t, personalSub.TrialEndsAt)
	require.Nil(t, subsByTeam[teamANoSub.ID])

	// User B owns one team with an on-trial subscription carrying trial_ends_at.
	require.Len(t, teamsByOwner[userB.ID], 1)
	trialSub := subsByTeam[teamB.ID]
	require.NotNil(t, trialSub)
	require.Equal(t, billingtypes.SubscriptionStatusOnTrial, trialSub.Status)
	require.NotNil(t, trialSub.TrialEndsAt)
}

func TestRegistry_CurrentSubscriptionsByTeams_PrefersActiveOverNewer(t *testing.T) {
	db := setupBillingDB(t)
	repo := NewRegistry(db)
	ctx := context.Background()

	team := authmodels.Team{Name: "Team", UserID: "owner-1"}
	require.NoError(t, db.Create(&team).Error)

	// Older active subscription, then a newer cancelled one. The active should
	// win even though it is not the newest by id.
	require.NoError(t, db.Create(&billingmodels.Subscription{
		BillableType:           billingmodels.BillableTypeTeam,
		BillableID:             team.ID,
		Type:                   "default",
		ProviderSubscriptionID: "sub_active",
		Status:                 billingtypes.SubscriptionStatusActive,
		ProductID:              "prod_1",
		VariantID:              "var_1",
	}).Error)
	require.NoError(t, db.Create(&billingmodels.Subscription{
		BillableType:           billingmodels.BillableTypeTeam,
		BillableID:             team.ID,
		Type:                   "default",
		ProviderSubscriptionID: "sub_cancelled",
		Status:                 billingtypes.SubscriptionStatusCancelled,
		ProductID:              "prod_1",
		VariantID:              "var_1",
	}).Error)

	subs, err := repo.CurrentSubscriptionsByTeams(ctx, []string{team.ID})
	require.NoError(t, err)
	require.NotNil(t, subs[team.ID])
	require.Equal(t, billingtypes.SubscriptionStatusActive, subs[team.ID].Status)
}

// baseModelAt builds a BaseModel with a fixed CreatedAt so ordering is
// deterministic in tests (BeforeCreate still assigns the ULID id).
func baseModelAt(at time.Time) basemodels.BaseModel {
	return basemodels.BaseModel{CreatedAt: &at, UpdatedAt: &at}
}
