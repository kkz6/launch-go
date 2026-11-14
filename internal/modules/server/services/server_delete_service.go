package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

// DeleteServer uses the same resource and lifecycle checks for team and staff
// requests. Custom machines are only disconnected from launchctl; cloud VMs
// are deleted by the provider job before their records are removed.
func (s *Service) DeleteServer(ctx context.Context, id, teamID, userID string) error {
	return s.deleteServer(ctx, id, teamID, userID, false)
}

// RemoveServerRecord removes launchctl's record without contacting the cloud
// provider or remote host. Only the explicit staff removal action exposes it.
func (s *Service) RemoveServerRecord(ctx context.Context, id, teamID, userID string) error {
	return s.deleteServer(ctx, id, teamID, userID, true)
}

// UninstallServer uses the existing deletion path when SSH is available and
// removes local records when the remote host cannot be reached.
func (s *Service) UninstallServer(ctx context.Context, id, teamID, userID string) (removedOffline bool, err error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return false, err
	}
	connection := server.ConnectionAsRoot()
	offline := connection.Host == "" || connection.PrivateKey == ""
	if !offline {
		client, err := connection.NewSSHClient(5 * time.Second)
		offline = err != nil
		if !offline {
			offline = client.ConnectContext(ctx) != nil
			_ = client.Close()
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return offline, s.deleteServer(ctx, id, teamID, userID, offline)
}

func (s *Service) deleteServer(ctx context.Context, id, teamID, userID string, removeRecord bool) error {
	if !s.HasDB() {
		return fiberutil.Internal("Database not configured")
	}
	server := &models.Server{}
	err := s.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The parent lock also blocks foreign-key inserts for new workloads
		// until the resource checks and deletion/status claim are committed.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND team_id = ?", id, teamID).First(server).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiberutil.NotFound()
			}
			return err
		}
		switch server.Status {
		case types.ServerStatusDeleting:
			return fiberutil.Conflict("Server deletion is already in progress")
		case types.ServerStatusStarting, types.ServerStatusProvisioning:
			return fiberutil.Conflict("Wait for server provisioning to finish before deleting it")
		}
		if removeRecord {
			var pendingSites int64
			if err := tx.Table("sites").Where("server_id = ? AND uninstallation_requested_at IS NOT NULL", id).
				Count(&pendingSites).Error; err != nil {
				return fmt.Errorf("count pending site uninstalls: %w", err)
			}
			if pendingSites > 0 {
				return fiberutil.Conflict("Wait for pending site uninstalls to finish before removing this server")
			}
			// Server foreign keys cascade through sites and Docker projects.
			return tx.Delete(server).Error
		}
		var siteCount int64
		if err := tx.Table("sites").Where("server_id = ?", id).Count(&siteCount).Error; err != nil {
			return fmt.Errorf("count server sites: %w", err)
		}
		if siteCount > 0 {
			return fiberutil.Validation("Remove every site before deleting this server")
		}
		if err := guardDockerProjectsExist(ctx, tx, id); err != nil {
			return err
		}
		if server.Provider == types.ProviderCustom {
			return tx.Delete(server).Error
		}
		if !s.HasQueue() {
			return ErrQueueNotConfigured
		}
		if err := tx.Model(&models.Server{}).Where("id = ?", id).
			Update("status", types.ServerStatusDeleting).Error; err != nil {
			return err
		}
		// The worker locks this same row, so it cannot act before commit.
		// A rejected enqueue rolls back the status change automatically.
		return s.dispatchDeleteJob(server, userID)
	})
	if err != nil {
		return err
	}
	if removeRecord {
		activity.RecordEvent(ctx, "removed", userID, server, "Server removed from launchctl; remote machine left unchanged")
	} else {
		activity.RecordEvent(ctx, "deletion_requested", userID, server, "Server deletion requested")
	}
	if removeRecord || server.Provider == types.ProviderCustom {
		s.BroadcastToTeam(server.TeamID, "server.deleted", map[string]any{"server_id": server.ID})
	} else if updated, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID); err == nil {
		s.broadcastServerUpdate(updated)
	}
	return nil
}
