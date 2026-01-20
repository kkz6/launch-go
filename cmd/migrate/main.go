package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/database/migrations"
	"github.com/kkz6/launch-go/internal/pkg/logger"
)

func main() {
	// Define command-line flags
	migrateCmd := flag.NewFlagSet("migrate", flag.ExitOnError)
	rollbackCmd := flag.NewFlagSet("rollback", flag.ExitOnError)
	freshCmd := flag.NewFlagSet("fresh", flag.ExitOnError)
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	syncCmd := flag.NewFlagSet("sync", flag.ExitOnError)
	syncUntil := syncCmd.String("until", "", "Sync only up to this migration ID")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	appLogger := logger.New(cfg.App.Environment)

	// Initialize database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Initialize migrator
	migrator := migrations.NewMigrator(db, appLogger)

	switch os.Args[1] {
	case "migrate", "up":
		_ = migrateCmd.Parse(os.Args[2:])
		if err := migrator.Migrate(); err != nil {
			appLogger.Fatal().Err(err).Msg("Migration failed")
		}
		appLogger.Info().Msg("Migration completed successfully")

	case "rollback", "down":
		_ = rollbackCmd.Parse(os.Args[2:])
		if err := migrator.Rollback(); err != nil {
			appLogger.Fatal().Err(err).Msg("Rollback failed")
		}
		appLogger.Info().Msg("Rollback completed successfully")

	case "fresh":
		_ = freshCmd.Parse(os.Args[2:])
		if err := migrator.Fresh(); err != nil {
			appLogger.Fatal().Err(err).Msg("Fresh migration failed")
		}
		appLogger.Info().Msg("Fresh migration completed successfully")

	case "status":
		_ = statusCmd.Parse(os.Args[2:])
		statuses, err := migrator.Status()
		if err != nil {
			appLogger.Fatal().Err(err).Msg("Failed to get migration status")
		}

		fmt.Println("\nMigration Status:")
		fmt.Println("------------------")
		for _, status := range statuses {
			ran := "Pending"
			if status.Ran {
				ran = "Ran"
			}
			fmt.Printf("[%s] %s - %s\n", ran, status.ID, status.Name)
		}
		fmt.Println()

	case "sync":
		_ = syncCmd.Parse(os.Args[2:])
		if *syncUntil != "" {
			if err := migrator.SyncUntil(*syncUntil); err != nil {
				appLogger.Fatal().Err(err).Msg("Sync failed")
			}
		} else {
			if err := migrator.Sync(); err != nil {
				appLogger.Fatal().Err(err).Msg("Sync failed")
			}
		}
		appLogger.Info().Msg("Sync completed successfully")

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Database Migration Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/migrate <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  migrate, up    Run all pending migrations")
	fmt.Println("  rollback, down Rollback the last batch of migrations")
	fmt.Println("  fresh          Drop all tables and re-run all migrations")
	fmt.Println("  status         Show the status of all migrations")
	fmt.Println("  sync           Mark all pending migrations as ran (without running them)")
	fmt.Println("                 Use --until=<migration_id> to sync up to a specific migration")
	fmt.Println()
}
