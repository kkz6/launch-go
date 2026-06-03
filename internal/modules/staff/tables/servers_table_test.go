package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/pkg/table"
)

// TestServersTableMeta asserts the declared schema: only the safe columns are
// present and NONE of the secret server columns are declared, so they can't
// surface in /meta or /data.
func TestServersTableMeta(t *testing.T) {
	tbl := NewServersTable()
	meta := table.Render(tbl)

	keys := make([]string, 0, len(meta.Columns))
	for _, c := range meta.Columns {
		keys = append(keys, c.Key)
	}

	require.ElementsMatch(t, []string{
		"name", "provider", "public_ipv4", "status", "created_at",
	}, keys)

	secrets := []string{
		"private_key", "public_key", "host_key", "user_public_key",
		"password", "database_password", "provider_data", "launch_token",
	}
	for _, s := range secrets {
		require.NotContains(t, keys, s, "secret column %q must not be declared", s)
	}

	// Read-only table: no row actions, hence no action column.
	require.Empty(t, meta.Actions.Row)
}

// TestServersTableQueryExcludesSecrets proves the model-driven query path only
// selects declared columns, so a secret private_key never reaches /data even
// though it lives on the same row. The Server model carries encrypted/JSON
// columns that don't AutoMigrate cleanly under sqlite, so we build a minimal
// servers table by hand with the columns the query touches plus a secret
// private_key column seeded with a real secret value.
func TestServersTableQueryExcludesSecrets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE servers (
			id          TEXT PRIMARY KEY,
			name        TEXT,
			provider    TEXT,
			public_ipv4 TEXT,
			status      TEXT,
			private_key TEXT,
			created_at  DATETIME
		)
	`).Error)

	require.NoError(t, db.Exec(`
		INSERT INTO servers (id, name, provider, public_ipv4, status, private_key, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		"01HZZZZZZZZZZZZZZZZZZZZZZZ",
		"prod-web-01",
		"hetzner",
		"203.0.113.10",
		"running",
		"-----BEGIN OPENSSH PRIVATE KEY-----super-secret-----END-----",
		"2026-01-01 00:00:00",
	).Error)

	svc := table.NewQueryService(db)
	tbl := NewServersTable()

	res, err := svc.Execute(context.Background(), tbl, table.Request{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, res.Data, 1)

	row := res.Data[0]
	require.Equal(t, "prod-web-01", row["name"])
	require.Equal(t, "203.0.113.10", row["public_ipv4"])

	secrets := []string{
		"private_key", "public_key", "host_key", "user_public_key",
		"password", "database_password", "provider_data", "launch_token",
	}
	for _, s := range secrets {
		require.NotContains(t, row, s, "secret column %q must not reach /data", s)
	}
}
