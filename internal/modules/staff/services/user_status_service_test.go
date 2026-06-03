package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

// setupUserStatusService returns a staff service backed by an in-memory sqlite
// DB holding just the users table, which AutoMigrates cleanly.
func setupUserStatusService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&authmodels.User{}))

	return NewService(repositories.NewRegistry(db)), db
}

func seedUser(t *testing.T, db *gorm.DB, user authmodels.User) string {
	t.Helper()
	if user.Status == "" {
		user.Status = authtypes.UserStatusActive
	}
	require.NoError(t, db.Create(&user).Error)
	return user.ID
}

func readStatus(t *testing.T, db *gorm.DB, userID string) authtypes.UserStatus {
	t.Helper()
	var stored authmodels.User
	require.NoError(t, db.First(&stored, "id = ?", userID).Error)
	return stored.Status
}

func TestSetUserStatus_SuspendAndUnsuspendNormalUser(t *testing.T) {
	svc, db := setupUserStatusService(t)
	ctx := context.Background()

	actor := seedUser(t, db, authmodels.User{Name: "Admin", Email: "admin@example.com"})
	target := seedUser(t, db, authmodels.User{Name: "Customer", Email: "customer@example.com"})

	require.NoError(t, svc.SetUserStatus(ctx, actor, target, authtypes.UserStatusSuspended))
	assert.Equal(t, authtypes.UserStatusSuspended, readStatus(t, db, target))

	require.NoError(t, svc.SetUserStatus(ctx, actor, target, authtypes.UserStatusActive))
	assert.Equal(t, authtypes.UserStatusActive, readStatus(t, db, target))
}

func TestSetUserStatus_CannotSuspendStaff(t *testing.T) {
	svc, db := setupUserStatusService(t)
	ctx := context.Background()

	actor := seedUser(t, db, authmodels.User{Name: "Admin", Email: "admin@example.com"})

	support := stafftypes.StaffRoleSupport
	target := seedUser(t, db, authmodels.User{
		Name:      "Support Staff",
		Email:     "support@example.com",
		StaffRole: &support,
	})

	err := svc.SetUserStatus(ctx, actor, target, authtypes.UserStatusSuspended)
	assert.ErrorIs(t, err, ErrCannotSuspendStaff)

	// Status must remain unchanged.
	assert.Equal(t, authtypes.UserStatusActive, readStatus(t, db, target))
}

func TestSetUserStatus_TargetNotFound(t *testing.T) {
	svc, db := setupUserStatusService(t)
	ctx := context.Background()

	actor := seedUser(t, db, authmodels.User{Name: "Admin", Email: "admin@example.com"})

	err := svc.SetUserStatus(ctx, actor, "nonexistent", authtypes.UserStatusSuspended)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestSetUserStatus_CannotSuspendSelf(t *testing.T) {
	svc, db := setupUserStatusService(t)
	ctx := context.Background()

	actor := seedUser(t, db, authmodels.User{Name: "Admin", Email: "admin@example.com"})

	err := svc.SetUserStatus(ctx, actor, actor, authtypes.UserStatusSuspended)
	assert.ErrorIs(t, err, ErrCannotSuspendSelf)

	assert.Equal(t, authtypes.UserStatusActive, readStatus(t, db, actor))
}
