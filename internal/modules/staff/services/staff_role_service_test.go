package services

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

func staffRoleTestDSN() string {
	if dsn := os.Getenv("STAFF_ROLE_TEST_DSN"); dsn != "" {
		return dsn
	}
	return os.Getenv("MIGRATION_TEST_DSN")
}

func staffRoleFixture(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	dialector := sqlite.Open(":memory:")
	schema := ""
	// Both DSNs must reference a disposable test database. PostgreSQL tests
	// isolate their schema and never read or mutate application users.
	if dsn := staffRoleTestDSN(); dsn != "" {
		schema = "staff_role_" + strings.ToLower(util.NewULID())
		if strings.Contains(dsn, "://") {
			parsed, err := url.Parse(dsn)
			require.NoError(t, err)
			query := parsed.Query()
			query.Set("search_path", schema)
			parsed.RawQuery = query.Encode()
			dsn = parsed.String()
		} else {
			dsn += " search_path=" + schema
		}
		dialector = postgres.Open(dsn)
	}
	db, err := gorm.Open(dialector, &gorm.Config{PrepareStmt: true, SkipDefaultTransaction: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := db.DB()
	require.NoError(t, err)
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	if schema != "" {
		connection.SetMaxOpenConns(8)
		require.NoError(t, db.Exec("CREATE SCHEMA "+schema).Error)
		t.Cleanup(func() { require.NoError(t, db.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
	}
	require.NoError(t, db.Exec(`CREATE TABLE users (id VARCHAR(26) PRIMARY KEY, name TEXT, email TEXT, password TEXT, staff_role VARCHAR(20), status VARCHAR(20), created_at TIMESTAMP, updated_at TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE teams (id VARCHAR(26) PRIMARY KEY, user_id VARCHAR(26))`).Error)
	require.NoError(t, db.AutoMigrate(&activity.ActivityLog{}))
	seedStaffRole(t, db, "admin-a", stafftypes.StaffRoleSuperAdmin)
	seedStaffRole(t, db, "target", stafftypes.StaffRoleSupport)
	return NewService(repositories.NewRegistry(db)), db
}

func seedStaffRole(t *testing.T, db *gorm.DB, id string, role stafftypes.StaffRole) {
	t.Helper()
	var value any
	if role != "" {
		value = role.String()
	}
	require.NoError(t, db.Table("users").Create(map[string]any{"id": id, "name": id, "email": id + "@example.test", "password": "unused", "staff_role": value, "status": "active"}).Error)
}

func TestStaffRoleDemotionIsAuditedAndAllowsDeletion(t *testing.T) {
	svc, db := staffRoleFixture(t)
	ctx := context.Background()
	allowed, _, err := svc.CanDeleteUser(ctx, "target")
	require.NoError(t, err)
	require.False(t, allowed)
	result, err := svc.UpdateStaffRole(ctx, "admin-a", "target", nil)
	require.NoError(t, err)
	require.Equal(t, "target", result.ID)
	require.Nil(t, result.StaffRole)
	require.Nil(t, svc.StaffRoleForUser(ctx, "target"))
	var audit activity.ActivityLog
	require.NoError(t, db.First(&audit).Error)
	require.Equal(t, "admin-a", *audit.CauserID)
	require.Equal(t, "target", *audit.SubjectID)
	require.Equal(t, "staff_role_updated", *audit.Event)
	require.Equal(t, "support", audit.Properties["old_staff_role"])
	require.Nil(t, audit.Properties["new_staff_role"])
	allowed, _, err = svc.CanDeleteUser(ctx, "target")
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, svc.DeleteUser(ctx, "admin-a", "target"))
	user, err := svc.repos.UserByID(ctx, "target")
	require.NoError(t, err)
	require.Nil(t, user)
}

func TestStaffRoleChangeGuards(t *testing.T) {
	for _, tc := range []struct {
		name, actor, target string
		role                stafftypes.StaffRole
		want                error
	}{
		{"self demotion", "admin-a", "admin-a", "", ErrCannotChangeOwnStaffRole},
		{"self role update", "admin-a", "admin-a", stafftypes.StaffRoleSupport, ErrCannotChangeOwnStaffRole},
		{"support actor", "target", "admin-a", "", ErrStaffRoleForbidden},
		{"missing actor", "missing", "target", "", ErrStaffRoleForbidden},
		{"missing target", "admin-a", "missing", "", ErrUserNotFound},
		{"invalid role", "admin-a", "target", "owner", ErrInvalidStaffRole},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := staffRoleFixture(t)
			var role *stafftypes.StaffRole
			if tc.role != "" {
				role = &tc.role
			}
			_, err := svc.UpdateStaffRole(context.Background(), tc.actor, tc.target, role)
			require.ErrorIs(t, err, tc.want)
			var admins, audits int64
			require.NoError(t, db.Model(&authmodels.User{}).Where("staff_role = ?", stafftypes.StaffRoleSuperAdmin).Count(&admins).Error)
			require.EqualValues(t, 1, admins)
			require.NoError(t, db.Model(&activity.ActivityLog{}).Count(&audits).Error)
			require.Zero(t, audits)
		})
	}
}

func TestStaffRolePromotionAndAuditFailureAreAtomic(t *testing.T) {
	svc, db := staffRoleFixture(t)
	role := stafftypes.StaffRoleSuperAdmin
	result, err := svc.UpdateStaffRole(context.Background(), "admin-a", "target", &role)
	require.NoError(t, err)
	require.Equal(t, role, *result.StaffRole)
	require.Equal(t, role, *svc.StaffRoleForUser(context.Background(), "target"))
	// The same-role request is idempotent and does not manufacture another audit.
	_, err = svc.UpdateStaffRole(context.Background(), "admin-a", "target", &role)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&activity.ActivityLog{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Migrator().DropTable(&activity.ActivityLog{}))
	_, err = svc.UpdateStaffRole(context.Background(), "admin-a", "target", nil)
	require.Error(t, err)
	require.Equal(t, role, *svc.StaffRoleForUser(context.Background(), "target"))
}

func TestStaffRoleDemotionPreservesPaidUserDeletionGuard(t *testing.T) {
	svc, db := setupUserDeleteService(t)
	require.NoError(t, db.AutoMigrate(&activity.ActivityLog{}))
	admin, support := stafftypes.StaffRoleSuperAdmin, stafftypes.StaffRoleSupport
	actor := seedUser(t, db, authmodels.User{Name: "Admin", Email: "admin-role@example.test", StaffRole: &admin})
	target := seedUser(t, db, authmodels.User{Name: "Customer", Email: "paid-role@example.test", StaffRole: &support})
	team := seedTeam(t, db, target)
	seedOrder(t, db, team, billingtypes.OrderStatusPaid)
	_, err := svc.UpdateStaffRole(context.Background(), actor, target, nil)
	require.NoError(t, err)
	allowed, reason, err := svc.CanDeleteUser(context.Background(), target)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, reasonPaidOrder, reason)
	err = svc.DeleteUser(context.Background(), actor, target)
	var blocked *NotDeletableError
	require.ErrorAs(t, err, &blocked)
	require.Equal(t, reasonPaidOrder, blocked.Reason)
}

func TestStaffRoleConcurrentDemotionsKeepOneAuthorizedAdminOnPostgres(t *testing.T) {
	if staffRoleTestDSN() == "" {
		t.Skip("set STAFF_ROLE_TEST_DSN to a disposable PostgreSQL database")
	}
	svc, db := staffRoleFixture(t)
	seedStaffRole(t, db, "admin-b", stafftypes.StaffRoleSuperAdmin)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, pair := range [][2]string{{"admin-a", "admin-b"}, {"admin-b", "admin-a"}} {
		go func() { <-start; _, err := svc.UpdateStaffRole(ctx, pair[0], pair[1], nil); results <- err }()
	}
	close(start)
	first, second := <-results, <-results
	require.True(t, first == nil && errors.Is(second, ErrStaffRoleForbidden) || second == nil && errors.Is(first, ErrStaffRoleForbidden))
	var admins, audits int64
	require.NoError(t, db.Model(&authmodels.User{}).Where("staff_role = ?", stafftypes.StaffRoleSuperAdmin).Count(&admins).Error)
	require.EqualValues(t, 1, admins)
	require.NoError(t, db.Model(&activity.ActivityLog{}).Count(&audits).Error)
	require.EqualValues(t, 1, audits)
}

func TestStaffRolePromotionCannotRaceUserDeletionOnPostgres(t *testing.T) {
	if staffRoleTestDSN() == "" {
		t.Skip("set STAFF_ROLE_TEST_DSN to a disposable PostgreSQL database")
	}
	svc, db := staffRoleFixture(t)
	require.NoError(t, db.Model(&authmodels.User{}).Where("id = ?", "target").Update("staff_role", nil).Error)
	updated, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:hold-role-audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "activity_log" {
			close(updated)
			<-release
		}
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	promoted, deleted := make(chan error, 1), make(chan error, 1)
	go func() {
		role := stafftypes.StaffRoleSuperAdmin
		_, err := svc.UpdateStaffRole(ctx, "admin-a", "target", &role)
		promoted <- err
	}()
	select {
	case <-updated:
	case err := <-promoted:
		t.Fatalf("promotion ended before audit barrier: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	go func() { deleted <- svc.DeleteUser(ctx, "admin-a", "target") }()
	select {
	case err := <-deleted:
		t.Fatalf("deletion did not wait for the role change: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	require.NoError(t, <-promoted)
	var blocked *NotDeletableError
	require.ErrorAs(t, <-deleted, &blocked)
	require.Equal(t, reasonStaffRole, blocked.Reason)
	require.Equal(t, stafftypes.StaffRoleSuperAdmin, *svc.StaffRoleForUser(ctx, "target"))
}
