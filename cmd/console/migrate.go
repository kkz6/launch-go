// migrate:* — database migration commands (run, rollback, fresh, status, sync).
// Faithful port of the former cmd/migrate entrypoint into the console system.
package main

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/database/migrations"
	"github.com/kkz6/launch-go/internal/pkg/console"
	"github.com/kkz6/launch-go/internal/pkg/logger"
)

const migrateCategory = "migrate"

// migrateCommands returns the full set of migrate console commands.
func migrateCommands() []console.Command {
	return []console.Command{
		&migrateRunCommand{},
		&migrateRollbackCommand{},
		&migrateFreshCommand{},
		&migrateStatusCommand{},
		&migrateSyncCommand{},
	}
}

// migrateBootstrap loads config, initialises the logger and database
// connection, and returns a ready-to-use Migrator. On any failure it writes an
// error via ctx and returns a non-nil error so the caller can return.
func migrateBootstrap(ctx console.Context) (*migrations.Migrator, error) {
	cfg, err := config.Load()
	if err != nil {
		ctx.Error(fmt.Sprintf("Failed to load config: %v", err))
		return nil, err
	}

	appLogger := logger.New(cfg.App.Environment)

	db, err := database.Connect(cfg.Database)
	if err != nil {
		ctx.Error(fmt.Sprintf("Failed to connect to database: %v", err))
		return nil, err
	}

	return migrations.NewMigrator(db, appLogger), nil
}

// migrate:run

type migrateRunCommand struct{}

func (migrateRunCommand) Signature() string      { return "migrate:run" }
func (migrateRunCommand) Description() string    { return "Run all pending database migrations" }
func (migrateRunCommand) Extend() console.Extend { return console.Extend{Category: migrateCategory} }

func (migrateRunCommand) Handle(ctx console.Context) error {
	migrator, err := migrateBootstrap(ctx)
	if err != nil {
		return err
	}
	if err := migrator.Migrate(); err != nil {
		ctx.Error(fmt.Sprintf("Migration failed: %v", err))
		return err
	}
	ctx.Success("Migration completed successfully")
	return nil
}

// migrate:rollback

type migrateRollbackCommand struct{}

func (migrateRollbackCommand) Signature() string { return "migrate:rollback" }
func (migrateRollbackCommand) Description() string {
	return "Disabled — migrations are up-only (rollback risks data loss)"
}
func (migrateRollbackCommand) Extend() console.Extend {
	return console.Extend{Category: migrateCategory}
}

// Handle refuses immediately. Migrations are up-only: rolling one back risks
// irreversible data loss, so there are no Down functions to run. We don't even
// connect to the database — there's nothing a rollback could safely do.
func (migrateRollbackCommand) Handle(ctx console.Context) error {
	ctx.Error("rollback is disabled: migrations are up-only to prevent data loss")
	return fmt.Errorf("rollback is disabled")
}

// migrate:fresh

type migrateFreshCommand struct{}

func (migrateFreshCommand) Signature() string      { return "migrate:fresh" }
func (migrateFreshCommand) Description() string    { return "Drop all tables and re-run all migrations" }
func (migrateFreshCommand) Extend() console.Extend { return console.Extend{Category: migrateCategory} }

func (migrateFreshCommand) Handle(ctx console.Context) error {
	migrator, err := migrateBootstrap(ctx)
	if err != nil {
		return err
	}
	if err := migrator.Fresh(); err != nil {
		ctx.Error(fmt.Sprintf("Fresh migration failed: %v", err))
		return err
	}
	ctx.Success("Fresh migration completed successfully")
	return nil
}

// migrate:status

type migrateStatusCommand struct{}

func (migrateStatusCommand) Signature() string      { return "migrate:status" }
func (migrateStatusCommand) Description() string    { return "Show the status of all database migrations" }
func (migrateStatusCommand) Extend() console.Extend { return console.Extend{Category: migrateCategory} }

func (migrateStatusCommand) Handle(ctx console.Context) error {
	migrator, err := migrateBootstrap(ctx)
	if err != nil {
		return err
	}
	statuses, err := migrator.Status()
	if err != nil {
		ctx.Error(fmt.Sprintf("Failed to get migration status: %v", err))
		return err
	}

	rows := make([][]string, len(statuses))
	for i, s := range statuses {
		state := "Pending"
		if s.Ran {
			state = "Ran"
		}
		rows[i] = []string{state, s.ID, s.Name}
	}

	ctx.NewLine()
	ctx.Table([]string{"Status", "ID", "Name"}, rows)
	ctx.NewLine()
	return nil
}

// migrate:sync

type migrateSyncCommand struct{}

func (migrateSyncCommand) Signature() string { return "migrate:sync" }
func (migrateSyncCommand) Description() string {
	return "Mark pending migrations as ran without executing them (use --until to stop at a specific ID)"
}
func (migrateSyncCommand) Extend() console.Extend {
	return console.Extend{
		Category: migrateCategory,
		Flags: []console.Flag{
			console.StringFlag{Name: "until", Usage: "Sync only up to this migration ID (inclusive)"},
		},
	}
}

func (migrateSyncCommand) Handle(ctx console.Context) error {
	migrator, err := migrateBootstrap(ctx)
	if err != nil {
		return err
	}

	until := ctx.Option("until")
	if until != "" {
		err = migrator.SyncUntil(until)
	} else {
		err = migrator.Sync()
	}
	if err != nil {
		ctx.Error(fmt.Sprintf("Sync failed: %v", err))
		return err
	}

	ctx.Success("Sync completed successfully")
	return nil
}
