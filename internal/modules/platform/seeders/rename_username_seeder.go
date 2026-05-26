package seeders

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"

	"github.com/kkz6/launch-go/internal/modules/platform/models"
	"github.com/kkz6/launch-go/internal/modules/platform/tasks"
	"github.com/kkz6/launch-go/internal/modules/platform/types"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// SeedRenameUsername creates the rename_username_captain platform update
// and generates server_platform_updates rows for all applicable servers
func SeedRenameUsername(ctx context.Context, db *gorm.DB, logger *zerolog.Logger) error {
	// Check if already exists. `key` is reserved in both MySQL and Postgres
	// but the dialects quote differently — use double quotes (SQL standard,
	// works in Postgres; works in MySQL when ANSI_QUOTES sql_mode is set,
	// which it is by default for Laravel/PDO clients).
	var count int64
	db.Model(&models.PlatformUpdate{}).Where(`"key" = ?`, tasks.RenameUsernameKey).Count(&count)
	if count > 0 {
		if logger != nil {
			logger.Debug().Msg("Platform update rename_username_captain already seeded")
		}
		return nil
	}

	// Create the platform update
	update := &models.PlatformUpdate{
		Key:         tasks.RenameUsernameKey,
		Title:       "Rename Default Username to Captain",
		Description: "The default UNIX username on provisioned servers is being renamed from `launcher` to `captain`. This update renames the system user, moves the home directory, and updates all related configuration files (Caddyfile, systemd services, crontab) on your servers.\n\nThis is a safe, idempotent operation — if a server already has the `captain` user, it will be skipped automatically.",
		Severity:    types.SeverityWarning,
	}
	update.ID = util.NewULID()

	if err := db.Create(update).Error; err != nil {
		return fmt.Errorf("failed to create platform update: %w", err)
	}

	// Find all provisioned servers where username is 'launcher' or NULL
	// Docker servers get status=skipped
	var servers []servermodels.Server
	if err := db.Where("provisioned_at IS NOT NULL AND archived_at IS NULL").Find(&servers).Error; err != nil {
		return fmt.Errorf("failed to find servers: %w", err)
	}

	for _, server := range servers {
		status := types.UpdateStatusPending

		// Docker servers get skipped
		if server.Type != nil && *server.Type == "docker" {
			status = types.UpdateStatusSkipped
		}

		// Skip servers that already have 'captain' username
		if server.Username != nil && *server.Username == "captain" {
			status = types.UpdateStatusSkipped
		}

		// Only create for servers with 'launcher' username or NULL (default)
		if server.Username != nil && *server.Username != "launcher" && *server.Username != "captain" {
			status = types.UpdateStatusSkipped
		}

		serverUpdate := &models.ServerPlatformUpdate{
			ServerID:         server.ID,
			PlatformUpdateID: update.ID,
			Status:           status,
		}
		serverUpdate.ID = util.NewULID()

		if err := db.Create(serverUpdate).Error; err != nil {
			if logger != nil {
				logger.Error().Err(err).
					Str("server_id", server.ID).
					Msg("Failed to create server platform update")
			}
			continue
		}
	}

	if logger != nil {
		logger.Info().
			Int("server_count", len(servers)).
			Msg("Seeded rename_username_captain platform update")
	}

	return nil
}
