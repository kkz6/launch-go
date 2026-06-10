package services

import (
	"context"
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
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// setupTrialService returns a staff service backed by an in-memory sqlite DB
// holding the platform_invitations + subscriptions tables for the trial-grant
// flow tests.
func setupTrialService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// A transaction opens a separate connection; with the default pool each
	// sqlite :memory: connection is an isolated database. Pin to a single
	// connection so the migrated tables are visible inside the transaction.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&authmodels.PlatformInvitation{},
		&billingmodels.Subscription{},
	))

	return NewService(repositories.NewRegistry(db)), db
}

// seedInvitation inserts a platform invitation and returns it.
func seedInvitation(t *testing.T, db *gorm.DB, mutate func(*authmodels.PlatformInvitation)) *authmodels.PlatformInvitation {
	t.Helper()

	now := time.Now()
	invitation := &authmodels.PlatformInvitation{
		ID:          util.NewULID(),
		Email:       "invitee@example.com",
		Token:       util.NewULID(),
		TrialEndsAt: now.Add(30 * 24 * time.Hour),
		InvitedBy:   "inviter-id",
		ExpiresAt:   now.Add(14 * 24 * time.Hour),
	}

	if mutate != nil {
		mutate(invitation)
	}

	require.NoError(t, db.Create(invitation).Error)

	return invitation
}

func TestAcceptPlatformInviteWithTrial_CreatesSubscriptionAndAcceptsInvite(t *testing.T) {
	svc, db := setupTrialService(t)
	ctx := context.Background()

	invitation := seedInvitation(t, db, nil)
	const teamID = "01TEAMTEAMTEAMTEAMTEAMTEAM"

	before := time.Now()
	err := svc.AcceptPlatformInviteWithTrial(ctx, invitation.Token, teamID)
	require.NoError(t, err)

	var sub billingmodels.Subscription
	require.NoError(t, db.Where("billable_id = ?", teamID).First(&sub).Error)
	assert.Equal(t, billingtypes.SubscriptionStatusOnTrial, sub.Status)
	assert.Equal(t, billingmodels.BillableTypeTeam, sub.BillableType)
	assert.Equal(t, teamID, sub.BillableID)
	require.NotNil(t, sub.TrialEndsAt)
	assert.WithinDuration(t, invitation.TrialEndsAt, *sub.TrialEndsAt, time.Second)
	assert.Equal(t, billingmodels.ProviderPolar, sub.Provider)

	var stored authmodels.PlatformInvitation
	require.NoError(t, db.Where("id = ?", invitation.ID).First(&stored).Error)
	require.NotNil(t, stored.AcceptedAt)
	assert.False(t, stored.AcceptedAt.Before(before.Add(-time.Second)))
}

func TestAcceptPlatformInviteWithTrial_RejectsExpiredOrAccepted(t *testing.T) {
	ctx := context.Background()

	t.Run("expired", func(t *testing.T) {
		svc, db := setupTrialService(t)
		invitation := seedInvitation(t, db, func(p *authmodels.PlatformInvitation) {
			p.ExpiresAt = time.Now().Add(-time.Hour)
		})

		err := svc.AcceptPlatformInviteWithTrial(ctx, invitation.Token, "team-x")
		require.Error(t, err)

		var count int64
		db.Model(&billingmodels.Subscription{}).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("already accepted", func(t *testing.T) {
		svc, db := setupTrialService(t)
		acceptedAt := time.Now().Add(-time.Hour)
		invitation := seedInvitation(t, db, func(p *authmodels.PlatformInvitation) {
			p.AcceptedAt = &acceptedAt
		})

		err := svc.AcceptPlatformInviteWithTrial(ctx, invitation.Token, "team-x")
		require.Error(t, err)

		var count int64
		db.Model(&billingmodels.Subscription{}).Count(&count)
		assert.Equal(t, int64(0), count)
	})
}

func TestPlatformInviteByToken(t *testing.T) {
	ctx := context.Background()

	t.Run("valid", func(t *testing.T) {
		svc, db := setupTrialService(t)
		invitation := seedInvitation(t, db, nil)

		email, trialEndsAt, ok, err := svc.PlatformInviteByToken(ctx, invitation.Token)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, invitation.Email, email)
		assert.WithinDuration(t, invitation.TrialEndsAt, trialEndsAt, time.Second)
	})

	t.Run("missing", func(t *testing.T) {
		svc, _ := setupTrialService(t)

		_, _, ok, err := svc.PlatformInviteByToken(ctx, "does-not-exist")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("expired", func(t *testing.T) {
		svc, db := setupTrialService(t)
		invitation := seedInvitation(t, db, func(p *authmodels.PlatformInvitation) {
			p.ExpiresAt = time.Now().Add(-time.Hour)
		})

		_, _, ok, err := svc.PlatformInviteByToken(ctx, invitation.Token)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("accepted", func(t *testing.T) {
		svc, db := setupTrialService(t)
		acceptedAt := time.Now().Add(-time.Hour)
		invitation := seedInvitation(t, db, func(p *authmodels.PlatformInvitation) {
			p.AcceptedAt = &acceptedAt
		})

		_, _, ok, err := svc.PlatformInviteByToken(ctx, invitation.Token)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
