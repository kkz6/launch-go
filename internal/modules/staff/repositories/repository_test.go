package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	staffmodels "github.com/kkz6/launch-go/internal/modules/staff/models"
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
