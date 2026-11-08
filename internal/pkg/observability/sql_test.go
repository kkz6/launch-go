package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRedactSQL(t *testing.T) {
	for _, tc := range []struct{ name, sql, dialect, want string }{
		{"postgres binds", `SELECT "users".* FROM "users" WHERE "id" = $1 LIMIT 1`, "postgres", `SELECT "users".* FROM "users" WHERE "id" = ? LIMIT ?`},
		{"literals and comments", "SELECT * FROM users WHERE token = 'secret''value' AND id = 123 /* password=hidden */ -- private\nLIMIT 5", "postgres", "SELECT * FROM users WHERE token = ? AND id = ? LIMIT ?"},
		{"escaped string", `SELECT E'secret\'value'`, "postgres", "SELECT E?"},
		{"postgres ordinary backslash", `SELECT '\' AS slash, 'secret-value' AS value`, "postgres", "SELECT ?"},
		{"postgres alternate escape mode", `SELECT 'secret\'value', 'another-secret'`, "postgres", "SELECT ?"},
		{"sqlite ordinary backslash", `SELECT '\' AS slash, 'secret-value' AS value`, "sqlite", "SELECT ? AS slash, ? AS value"},
		{"mysql ambiguous escape mode", `SELECT '\' AS slash, 'secret-value' AS value`, "mysql", "SELECT ?"},
		{"dollar strings", `SELECT $$secret$$, $tag$another secret$tag$, $1`, "postgres", "SELECT ?, ?, ?"},
		{"mysql", "SELECT `users`.* FROM `users` WHERE password = \"secret\" # secret\n AND id = 0x123abc", "mysql", "SELECT `users`.* FROM `users` WHERE password = ? AND id = ?"},
		{"nested comments", "SELECT 1 /* public /* private */ secret */", "postgres", "SELECT ?"},
		{"unterminated literal", "SELECT 'secret", "postgres", "SELECT ?"},
		{"identifier digits", "SELECT sha256, field_2 FROM users2 WHERE id = -12.5e+4", "postgres", "SELECT sha256, field_2 FROM users2 WHERE id = -?"},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, RedactSQL(tc.sql, tc.dialect)) })
	}
	redacted := RedactSQL("SELECT '"+strings.Repeat("private", 10000), "postgres")
	require.NotContains(t, redacted, "private")
	require.LessOrEqual(t, len(redacted), maxSQLBytes)
}

type queryStateError struct{ state string }

func (e queryStateError) Error() string    { return "duplicate secret@example.com" }
func (e queryStateError) SQLState() string { return e.state }

func TestQueryErrorRetainsOnlySafeCategories(t *testing.T) {
	require.Empty(t, QueryError(nil))
	require.Equal(t, "record not found", QueryError(gorm.ErrRecordNotFound))
	require.Equal(t, "context canceled", QueryError(context.Canceled))
	require.Equal(t, "deadline exceeded", QueryError(context.DeadlineExceeded))
	require.Equal(t, "query failed", QueryError(errors.New("password=private")))
	require.Equal(t, "query failed (SQLSTATE 23505)", QueryError(fmt.Errorf("wrapper: %w", queryStateError{"23505"})))
	require.Equal(t, "query failed", QueryError(queryStateError{"secret"}))
}
