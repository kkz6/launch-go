package tables

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

// setupUsersDB migrates users, teams and subscriptions on an in-memory sqlite
// DB. Subscription and Order share a database-global idx_billable index name on
// sqlite, so migrate each model independently and tolerate the collision (the
// table is still created).
func setupUsersDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	for _, model := range []any{
		&authmodels.User{},
		&authmodels.Team{},
		&billingmodels.Subscription{},
	} {
		err := db.AutoMigrate(model)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			require.NoError(t, err)
		}
	}

	return db
}

func buildUsersTable(db *gorm.DB) *UsersTable {
	return NewUsersTable(services.NewService(repositories.NewRegistry(db)))
}

func TestUsersTableMeta(t *testing.T) {
	tbl := NewUsersTable(nil)
	meta := table.Render(tbl)

	cols := map[string]bool{}
	for _, c := range meta.Columns {
		cols[c.Key] = true
	}
	require.True(t, cols["name"], "name column present")
	require.True(t, cols["email"], "email column present")
	require.True(t, cols["teams"], "teams column present")
	require.True(t, cols["staff_role"], "staff_role column present")
	require.True(t, cols["status"], "status column present")
	require.True(t, cols["created_at"], "created_at column present")
	require.True(t, cols["_actions"], "action column present")

	actions := map[string]bool{}
	for _, a := range meta.Actions.Row {
		actions[a.Name] = true
	}
	require.True(t, actions["suspend"], "suspend action present")
	require.True(t, actions["unsuspend"], "unsuspend action present")
	require.True(t, actions["delete"], "delete action present")
	require.False(t, actions["spectate"], "no spectate server action")

	require.Empty(t, meta.Actions.Bulk)
}

func TestUsersResolve(t *testing.T) {
	db := setupUsersDB(t)
	tbl := buildUsersTable(db)
	ctx := context.Background()

	user := authmodels.User{Name: "Customer", Email: "customer@example.com", Status: authtypes.UserStatusActive}
	require.NoError(t, db.Create(&user).Error)

	team := authmodels.Team{UserID: user.ID, Name: "Acme", PersonalTeam: true}
	require.NoError(t, db.Create(&team).Error)

	sub := billingmodels.Subscription{
		BillableType: billingmodels.BillableTypeTeam,
		BillableID:   team.ID,
		Status:       billingtypes.SubscriptionStatusActive,
	}
	require.NoError(t, db.Create(&sub).Error)

	resp, err := tbl.Resolve(ctx, table.Request{PerPage: 20})
	require.NoError(t, err)

	require.Equal(t, int64(1), resp.Pagination.Total)
	require.Equal(t, 1, resp.Pagination.CurrentPage)
	require.Equal(t, 20, resp.Pagination.PerPage)
	require.Len(t, resp.Data, 1)

	row := resp.Data[0]
	require.Equal(t, user.ID, row["id"])
	require.Equal(t, "Customer", row["name"])
	require.Equal(t, "customer@example.com", row["email"])

	teams, ok := row["teams"].([]map[string]any)
	require.True(t, ok, "teams is a []map[string]any")
	require.Len(t, teams, 1)
	require.Equal(t, "Acme", teams[0]["name"])
	require.Equal(t, true, teams[0]["personal_team"])

	subMap, ok := teams[0]["subscription"].(map[string]any)
	require.True(t, ok, "team carries a subscription map")
	require.Equal(t, billingtypes.SubscriptionStatusActive.String(), subMap["status"])

	// Meta schema travels with the data response.
	require.NotEmpty(t, resp.Meta.Columns)
}

func TestUsersActions(t *testing.T) {
	db := setupUsersDB(t)
	tbl := buildUsersTable(db)

	actor := authmodels.User{Name: "Admin", Email: "admin@example.com", Status: authtypes.UserStatusActive}
	require.NoError(t, db.Create(&actor).Error)
	target := authmodels.User{Name: "Customer", Email: "customer@example.com", Status: authtypes.UserStatusActive}
	require.NoError(t, db.Create(&target).Error)

	var suspend, unsuspend, del *table.Action
	for _, a := range tbl.Actions() {
		switch a.Name() {
		case "suspend":
			suspend = a
		case "unsuspend":
			unsuspend = a
		case "delete":
			del = a
		}
	}
	require.NotNil(t, suspend)
	require.NotNil(t, unsuspend)
	require.NotNil(t, del)

	// Hidden conditions: Suspend hidden once suspended; Unsuspend shown only when
	// suspended.
	require.True(t, suspend.IsHiddenFor(map[string]any{"status": "suspended"}))
	require.False(t, suspend.IsHiddenFor(map[string]any{"status": "active"}))
	require.True(t, unsuspend.IsHiddenFor(map[string]any{"status": "active"}))
	require.False(t, unsuspend.IsHiddenFor(map[string]any{"status": "suspended"}))

	// Suspend handler reads the actor from the context and flips the target's
	// status.
	ctx := table.WithActorID(context.Background(), actor.ID)
	require.NoError(t, suspend.Handler()(ctx, []string{target.ID}))

	var stored authmodels.User
	require.NoError(t, db.First(&stored, "id = ?", target.ID).Error)
	require.Equal(t, authtypes.UserStatusSuspended, stored.Status)

	// Unsuspend flips it back.
	require.NoError(t, unsuspend.Handler()(ctx, []string{target.ID}))
	require.NoError(t, db.First(&stored, "id = ?", target.ID).Error)
	require.Equal(t, authtypes.UserStatusActive, stored.Status)

	// Self-guard: the actor can't suspend themselves.
	require.ErrorIs(t, suspend.Handler()(ctx, []string{actor.ID}), services.ErrCannotSuspendSelf)
}
