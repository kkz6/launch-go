package services

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

// setupUserDeleteService returns a staff service backed by an in-memory sqlite
// DB with the users, teams, orders and subscriptions tables migrated.
//
// Subscription and Order both declare a composite index named idx_billable.
// sqlite index names are database-global (MySQL scopes them per-table), so
// migrating both in one call trips a duplicate-index error on the second table.
// Migrate each model independently and tolerate that specific collision — the
// table itself is still created.
func setupUserDeleteService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	for _, model := range []any{
		&authmodels.User{},
		&authmodels.Team{},
		&billingmodels.Subscription{},
		&billingmodels.Order{},
	} {
		err := db.AutoMigrate(model)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			require.NoError(t, err)
		}
	}

	return NewService(repositories.NewRegistry(db)), db
}

func seedTeam(t *testing.T, db *gorm.DB, ownerID string) string {
	t.Helper()
	team := authmodels.Team{UserID: ownerID, Name: "Team"}
	require.NoError(t, db.Create(&team).Error)
	return team.ID
}

func seedOrder(t *testing.T, db *gorm.DB, teamID string, status billingtypes.OrderStatus) {
	t.Helper()
	order := billingmodels.Order{
		BillableType:    billingmodels.BillableTypeTeam,
		BillableID:      teamID,
		Provider:        "dodo_payments",
		ProviderOrderID: "po_" + teamID + string(status),
		Identifier:      "id_" + teamID + string(status),
		ProductID:       "prod_1",
		VariantID:       "var_1",
		Currency:        "USD",
		Status:          status,
	}
	require.NoError(t, db.Create(&order).Error)
}

func seedSubscription(t *testing.T, db *gorm.DB, teamID string, status billingtypes.SubscriptionStatus) {
	t.Helper()
	sub := billingmodels.Subscription{
		BillableType:           billingmodels.BillableTypeTeam,
		BillableID:             teamID,
		Type:                   "default",
		Provider:               "dodo_payments",
		ProviderSubscriptionID: "sub_" + teamID + string(status),
		Status:                 status,
		ProductID:              "prod_1",
		VariantID:              "var_1",
	}
	require.NoError(t, db.Create(&sub).Error)
}

func userExists(t *testing.T, db *gorm.DB, userID string) bool {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&authmodels.User{}).Where("id = ?", userID).Count(&count).Error)
	return count > 0
}

func TestCanDeleteUser_CleanUserIsDeletable(t *testing.T) {
	svc, db := setupUserDeleteService(t)
	ctx := context.Background()

	user := seedUser(t, db, authmodels.User{Name: "Clean", Email: "clean@example.com"})
	teamID := seedTeam(t, db, user)
	// On-trial only: never paid, so deletable.
	seedSubscription(t, db, teamID, billingtypes.SubscriptionStatusOnTrial)

	deletable, reason, err := svc.CanDeleteUser(ctx, user)
	require.NoError(t, err)
	assert.True(t, deletable)
	assert.Empty(t, reason)
}

func TestDeleteUser_RemovesCleanUser(t *testing.T) {
	svc, db := setupUserDeleteService(t)
	ctx := context.Background()

	actor := seedUser(t, db, authmodels.User{Name: "Admin", Email: "admin@example.com"})
	user := seedUser(t, db, authmodels.User{Name: "Clean", Email: "clean@example.com"})
	seedTeam(t, db, user)

	require.NoError(t, svc.DeleteUser(ctx, actor, user))
	assert.False(t, userExists(t, db, user), "clean user row must be gone after delete")
}

func TestCanDeleteUser_PaidOrderBlocks(t *testing.T) {
	svc, db := setupUserDeleteService(t)
	ctx := context.Background()

	user := seedUser(t, db, authmodels.User{Name: "Payer", Email: "payer@example.com"})
	teamID := seedTeam(t, db, user)
	seedOrder(t, db, teamID, billingtypes.OrderStatusPaid)

	deletable, reason, err := svc.CanDeleteUser(ctx, user)
	require.NoError(t, err)
	assert.False(t, deletable)
	assert.Contains(t, reason, "paid order")

	// DeleteUser must also refuse and leave the row intact.
	err = svc.DeleteUser(ctx, "admin", user)
	nde, ok := asNotDeletable(err)
	require.True(t, ok, "expected NotDeletableError, got %v", err)
	assert.Contains(t, nde.Reason, "paid order")
	assert.True(t, userExists(t, db, user), "user with a paid order must survive a blocked delete")
}

func TestCanDeleteUser_ActiveSubscriptionBlocks(t *testing.T) {
	svc, db := setupUserDeleteService(t)
	ctx := context.Background()

	user := seedUser(t, db, authmodels.User{Name: "Subbed", Email: "subbed@example.com"})
	teamID := seedTeam(t, db, user)
	seedSubscription(t, db, teamID, billingtypes.SubscriptionStatusActive)

	deletable, reason, err := svc.CanDeleteUser(ctx, user)
	require.NoError(t, err)
	assert.False(t, deletable)
	assert.Contains(t, reason, "subscription")

	err = svc.DeleteUser(ctx, "admin", user)
	_, ok := asNotDeletable(err)
	require.True(t, ok, "expected NotDeletableError, got %v", err)
	assert.True(t, userExists(t, db, user))
}

func TestCanDeleteUser_CancelledSubscriptionBlocks(t *testing.T) {
	svc, db := setupUserDeleteService(t)
	ctx := context.Background()

	user := seedUser(t, db, authmodels.User{Name: "Churned", Email: "churned@example.com"})
	teamID := seedTeam(t, db, user)
	seedSubscription(t, db, teamID, billingtypes.SubscriptionStatusCancelled)

	deletable, reason, err := svc.CanDeleteUser(ctx, user)
	require.NoError(t, err)
	assert.False(t, deletable)
	assert.Contains(t, reason, "subscription")
}

func TestCanDeleteUser_StaffRoleBlocks(t *testing.T) {
	svc, db := setupUserDeleteService(t)
	ctx := context.Background()

	support := stafftypes.StaffRoleSupport
	user := seedUser(t, db, authmodels.User{
		Name:      "Staffer",
		Email:     "staffer@example.com",
		StaffRole: &support,
	})

	deletable, reason, err := svc.CanDeleteUser(ctx, user)
	require.NoError(t, err)
	assert.False(t, deletable)
	assert.Contains(t, reason, "staff role")

	err = svc.DeleteUser(ctx, "admin", user)
	_, ok := asNotDeletable(err)
	require.True(t, ok, "expected NotDeletableError, got %v", err)
	assert.True(t, userExists(t, db, user))
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	svc, db := setupUserDeleteService(t)
	ctx := context.Background()

	actor := seedUser(t, db, authmodels.User{Name: "Admin", Email: "admin@example.com"})
	seedTeam(t, db, actor)

	err := svc.DeleteUser(ctx, actor, actor)
	assert.ErrorIs(t, err, ErrCannotDeleteSelf)
	assert.True(t, userExists(t, db, actor), "self-delete must leave the actor intact")
}

func TestCanDeleteUser_NonexistentIsNotFound(t *testing.T) {
	svc, _ := setupUserDeleteService(t)
	ctx := context.Background()

	deletable, _, err := svc.CanDeleteUser(ctx, "nonexistent")
	assert.False(t, deletable)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestDeleteUser_NonexistentIsNotFound(t *testing.T) {
	svc, _ := setupUserDeleteService(t)
	ctx := context.Background()

	err := svc.DeleteUser(ctx, "admin", "nonexistent")
	assert.ErrorIs(t, err, ErrUserNotFound)
}
