package migrations

import (
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
)

// TestMigrationsApplyCleanlyOnPostgres applies the full registered migration set
// forward against a real Postgres database.
//
// Migrations are up-only by policy (no Down — a rollback risks data loss), so
// this guard only exercises the forward path: every migration applies cleanly
// and leaves nothing pending. The migrations are Postgres-specific (TIMESTAMPTZ,
// CHAR(26), JSON ->>, UPDATE ... FROM), so the SQLite-backed unit tests never
// exercise them. Without this guard a broken or missing migration only surfaces
// in production — which is exactly how the missing source_controls.token_expires_at
// column slipped through: a model gained a field that no migration added.
//
// The test connects through database.Connect so it inherits production's GORM
// configuration (PrepareStmt in particular — bundling multiple statements into
// one Exec under prepared statements is what broke an earlier deploy).
//
// Skipped unless MIGRATION_TEST_DSN points at a Postgres database. It RESETS the
// public schema, so point it ONLY at a disposable database. CI provides one via
// the postgres service in ci.yml.
func TestMigrationsApplyCleanlyOnPostgres(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set MIGRATION_TEST_DSN (a disposable Postgres URL) to run the migration integration test")
	}

	cfg := config.DatabaseConfig{URL: dsn}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	// Clean slate. PrepareStmt is on (same as production), and it rejects
	// multiple statements in a single Exec, so DROP and CREATE go separately.
	if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
		t.Fatalf("reset schema (drop): %v", err)
	}
	if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
		t.Fatalf("reset schema (create): %v", err)
	}

	nop := zerolog.Nop()
	migrator := NewMigrator(db, &nop)

	// Forward: every registered migration applies without error.
	if err := migrator.Migrate(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pending, err := migrator.GetPendingMigrations()
	if err != nil {
		t.Fatalf("list pending migrations: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending migrations after migrate, got %d", len(pending))
	}

	var localeNullable string
	if err := db.Raw(`
		SELECT is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'locale'
	`).Scan(&localeNullable).Error; err != nil {
		t.Fatalf("inspect users.locale: %v", err)
	}
	if localeNullable != "YES" {
		t.Fatalf("expected nullable users.locale column, got is_nullable=%q", localeNullable)
	}
}
