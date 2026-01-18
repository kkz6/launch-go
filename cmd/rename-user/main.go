package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	serverconfig "github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/queue"
)

func main() {
	// Define command-line flags
	oldUsername := flag.String("old", "launch", "The old username to rename from")
	newUsername := flag.String("new", serverconfig.DefaultUsername, "The new username to rename to")
	serverIDs := flag.String("server", "", "Comma-separated list of server IDs to rename (optional, defaults to all servers with old username)")
	serverNames := flag.String("name", "", "Comma-separated list of server names to rename (optional)")
	dryRun := flag.Bool("dry-run", false, "Show what would be done without making changes")
	force := flag.Bool("force", false, "Skip confirmation prompt")
	flag.Parse()

	if *oldUsername == *newUsername {
		fmt.Println("Error: old and new usernames are the same")
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

	// Initialize database encryption
	if err := database.InitEncryption(cfg.App.Key); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to initialize encryption")
	}

	// Initialize database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	ctx := context.Background()

	// Build query based on flags
	var servers []models.Server
	query := db.WithContext(ctx).Where("archived_at IS NULL")

	// Filter by specific server IDs if provided
	if *serverIDs != "" {
		ids := parseCommaSeparated(*serverIDs)
		query = query.Where("id IN ?", ids)
	}

	// Filter by server names if provided
	if *serverNames != "" {
		names := parseCommaSeparated(*serverNames)
		query = query.Where("name IN ?", names)
	}

	// If no specific servers specified, filter by old username
	if *serverIDs == "" && *serverNames == "" {
		query = query.Where("username = ?", *oldUsername)
	}

	if err := query.Find(&servers).Error; err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to query servers")
	}

	if len(servers) == 0 {
		if *serverIDs != "" || *serverNames != "" {
			fmt.Println("No servers found matching the specified criteria")
		} else {
			fmt.Printf("No servers found with username '%s'\n", *oldUsername)
		}
		os.Exit(0)
	}

	fmt.Printf("Found %d server(s):\n\n", len(servers))
	for _, server := range servers {
		ip := "N/A"
		if server.PublicIPv4 != nil {
			ip = *server.PublicIPv4
		}
		currentUsername := "N/A"
		if server.Username != nil {
			currentUsername = *server.Username
		}
		fmt.Printf("  - %s (ID: %s, IP: %s, Current User: %s)\n", server.Name, server.ID, ip, currentUsername)
	}
	fmt.Println()

	if *dryRun {
		fmt.Println("Dry run mode - no changes will be made")
		fmt.Printf("Would rename user from '%s' to '%s' on %d server(s)\n", *oldUsername, *newUsername, len(servers))
		os.Exit(0)
	}

	// Confirm action
	if !*force {
		fmt.Printf("This will rename the local user from '%s' to '%s' on %d server(s).\n", *oldUsername, *newUsername, len(servers))
		fmt.Print("Are you sure you want to continue? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted")
			os.Exit(0)
		}
	}

	// Initialize queue client
	queueClient := queue.NewClient(cfg.Redis)
	defer queueClient.Close()

	// Enqueue jobs for each server
	fmt.Println("\nEnqueuing rename jobs...")
	successCount := 0
	for _, server := range servers {
		// Determine the old username for this specific server
		serverOldUsername := *oldUsername
		if server.Username != nil && *server.Username != "" {
			serverOldUsername = *server.Username
		}

		// Skip if usernames are already the same
		if serverOldUsername == *newUsername {
			fmt.Printf("  - Skipping server '%s' (already has username '%s')\n", server.Name, *newUsername)
			continue
		}

		task, err := jobs.NewRenameLocalUserTask(server.ID, serverOldUsername, *newUsername)
		if err != nil {
			appLogger.Error().Err(err).Str("server_id", server.ID).Msg("Failed to create task")
			continue
		}

		if _, err := queueClient.Enqueue(task); err != nil {
			appLogger.Error().Err(err).Str("server_id", server.ID).Msg("Failed to enqueue task")
			continue
		}

		successCount++
		fmt.Printf("  ✓ Enqueued job for server '%s' (ID: %s, %s -> %s)\n", server.Name, server.ID, serverOldUsername, *newUsername)
	}

	fmt.Printf("\nSuccessfully enqueued %d/%d rename jobs\n", successCount, len(servers))
	fmt.Println("\nNote: Jobs will be processed by the worker. Monitor the worker logs for progress.")
}

// parseCommaSeparated splits a comma-separated string and trims whitespace
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
