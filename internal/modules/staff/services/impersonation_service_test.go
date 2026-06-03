package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	staffmodels "github.com/kkz6/launch-go/internal/modules/staff/models"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

const testJWTSecret = "test-impersonation-secret"

// setupImpersonationService builds a staff service backed by an in-memory sqlite
// DB with the impersonation_sessions and users tables migrated, plus a seeded
// target user. The User model uses only standard column types, so it migrates
// cleanly under sqlite without stubbing.
func setupImpersonationService(t *testing.T) (*Service, *authmodels.User, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&staffmodels.ImpersonationSession{}, &authmodels.User{}))

	teamID := "team-1"
	target := &authmodels.User{
		Name:          "Target Customer",
		Email:         "target@example.com",
		Password:      "hashed",
		CurrentTeamID: &teamID,
	}
	require.NoError(t, db.Create(target).Error)
	require.NotEmpty(t, target.ID)

	svc := NewService(repositories.NewRegistry(db))
	svc.SetJWTSecret(testJWTSecret)

	return svc, target, db
}

// activeImpersonationCount counts a staffer's currently-active sessions.
func activeImpersonationCount(t *testing.T, db *gorm.DB, staffID string) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.
		Model(&staffmodels.ImpersonationSession{}).
		Where("staff_id = ? AND ended_at IS NULL", staffID).
		Count(&count).Error)

	return count
}

func TestService_StartImpersonation_MintsScopedTokenAndRecordsSession(t *testing.T) {
	svc, target, _ := setupImpersonationService(t)
	ctx := context.Background()

	before := time.Now()
	token, session, err := svc.StartImpersonation(ctx, "staff1", target.ID, "ticket #1")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotNil(t, session)
	require.NotEmpty(t, session.ID)

	active, err := svc.repos.ActiveImpersonationForStaff(ctx, "staff1")
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, session.ID, active.ID)
	require.Equal(t, target.ID, active.TargetUserID)
	require.NotNil(t, active.Reason)
	require.Equal(t, "ticket #1", *active.Reason)
	require.NotNil(t, active.TeamID)
	require.Equal(t, "team-1", *active.TeamID)

	claims, err := security.ParseJWTToken(token, testJWTSecret)
	require.NoError(t, err)
	require.Equal(t, target.ID, claims["sub"])
	require.Equal(t, target.Email, claims["email"])
	require.Equal(t, "staff1", claims["impersonator_id"])
	require.Equal(t, session.ID, claims["impersonation_sid"])
	require.Equal(t, true, claims["read_only"])
	require.Equal(t, "access", claims["type"])

	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	expTime := time.Unix(int64(exp), 0)
	require.True(t, expTime.After(before), "exp must be in the future")
	require.True(t, expTime.Before(before.Add(31*time.Minute)), "exp must be ~30 min out")

	require.NotContains(t, claims, "team_id", "impersonation token must match normal access token shape (no team_id)")
}

func TestService_StopImpersonation_EndsActiveSession(t *testing.T) {
	svc, target, _ := setupImpersonationService(t)
	ctx := context.Background()

	_, session, err := svc.StartImpersonation(ctx, "staff1", target.ID, "")
	require.NoError(t, err)

	require.NoError(t, svc.StopImpersonation(ctx, session.ID))

	active, err := svc.repos.ActiveImpersonationForStaff(ctx, "staff1")
	require.NoError(t, err)
	require.Nil(t, active)
}

func TestService_StartImpersonation_EndsPriorActiveSession(t *testing.T) {
	svc, target, db := setupImpersonationService(t)
	ctx := context.Background()

	_, first, err := svc.StartImpersonation(ctx, "staff1", target.ID, "")
	require.NoError(t, err)

	_, second, err := svc.StartImpersonation(ctx, "staff1", target.ID, "")
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)

	require.Equal(t, int64(1), activeImpersonationCount(t, db, "staff1"),
		"starting a new session must end any prior active session")

	active, err := svc.repos.ActiveImpersonationForStaff(ctx, "staff1")
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, second.ID, active.ID)
}

func TestService_StopImpersonationForStaff_EndsAllActiveSessions(t *testing.T) {
	svc, target, db := setupImpersonationService(t)
	ctx := context.Background()

	// Force two concurrently-active rows for the same staffer by inserting
	// directly, bypassing the start-time single-active enforcement. This
	// simulates a pre-existing dirty audit trail.
	first := &staffmodels.ImpersonationSession{StaffID: "staff1", TargetUserID: target.ID}
	require.NoError(t, svc.repos.CreateImpersonationSession(ctx, first))

	second := &staffmodels.ImpersonationSession{StaffID: "staff1", TargetUserID: target.ID}
	require.NoError(t, svc.repos.CreateImpersonationSession(ctx, second))

	require.Equal(t, int64(2), activeImpersonationCount(t, db, "staff1"),
		"precondition: two active sessions")

	stopped, err := svc.StopImpersonationForStaff(ctx, "staff1")
	require.NoError(t, err)
	require.True(t, stopped)

	require.Equal(t, int64(0), activeImpersonationCount(t, db, "staff1"),
		"stopping must close ALL active sessions, leaving no orphan")

	active, err := svc.repos.ActiveImpersonationForStaff(ctx, "staff1")
	require.NoError(t, err)
	require.Nil(t, active)

	stopped, err = svc.StopImpersonationForStaff(ctx, "staff1")
	require.NoError(t, err)
	require.False(t, stopped, "no active sessions remain to stop")
}

func TestService_StartImpersonation_Validations(t *testing.T) {
	svc, target, _ := setupImpersonationService(t)
	ctx := context.Background()

	_, _, err := svc.StartImpersonation(ctx, "", target.ID, "")
	require.ErrorIs(t, err, ErrInvalidImpersonationRequest)

	_, _, err = svc.StartImpersonation(ctx, "staff1", "", "")
	require.ErrorIs(t, err, ErrInvalidImpersonationRequest)

	_, _, err = svc.StartImpersonation(ctx, "staff1", "nonexistent-user", "")
	require.ErrorIs(t, err, ErrTargetUserNotFound)
}
