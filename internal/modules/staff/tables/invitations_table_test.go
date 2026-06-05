package tables

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

// TestInvitationsTableMeta asserts the declared schema: the expected columns
// are present, the action column and revoke action exist, and the secret
// token is never declared (so it can't surface in /meta or /data).
func TestInvitationsTableMeta(t *testing.T) {
	tbl := NewInvitationsTable(nil)
	meta := table.Render(tbl)

	keys := make([]string, 0, len(meta.Columns))
	for _, c := range meta.Columns {
		keys = append(keys, c.Key)
	}

	require.ElementsMatch(t, []string{
		"email", "plan_id", "trial_ends_at", "expires_at", "accepted_at", "_actions",
	}, keys)

	require.NotContains(t, keys, "token")

	require.Len(t, meta.Actions.Row, 1)
	require.Equal(t, "revoke", meta.Actions.Row[0].Name)
	require.NotNil(t, meta.Actions.Row[0].Confirm)
}

// TestInvitationsTableQueryExcludesToken proves the model-driven query path
// only selects declared columns, so the invite token never reaches /data even
// though it lives on the same row.
func TestInvitationsTableQueryExcludesToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&authmodels.PlatformInvitation{}))

	now := time.Now()
	require.NoError(t, db.Create(&authmodels.PlatformInvitation{
		ID:          "01HZZZZZZZZZZZZZZZZZZZZZZZ",
		Email:       "invitee@example.com",
		Token:       "super-secret-token",
		TrialEndsAt: now.Add(30 * 24 * time.Hour),
		InvitedBy:   "01HAAAAAAAAAAAAAAAAAAAAAAA",
		ExpiresAt:   now.Add(14 * 24 * time.Hour),
	}).Error)

	svc := table.NewQueryService(db)
	tbl := NewInvitationsTable(nil)

	res, err := svc.Execute(context.Background(), tbl, table.Request{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, res.Data, 1)

	row := res.Data[0]
	require.Equal(t, "invitee@example.com", row["email"])
	require.NotContains(t, row, "token")
}
