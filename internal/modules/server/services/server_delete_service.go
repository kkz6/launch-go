package services

import (
	"context"
	"errors"
	"fmt"

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
	activity.RecordEvent(ctx, "deletion_requested", userID, server, "Server deletion requested")
	if server.Provider == types.ProviderCustom {
		s.BroadcastToTeam(server.TeamID, "server.deleted", map[string]any{"server_id": server.ID})
	} else if updated, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID); err == nil {
		s.broadcastServerUpdate(updated)
	}
	return nil
}
