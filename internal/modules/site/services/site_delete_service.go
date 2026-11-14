package services

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

// DeleteSite queues the existing uninstall workflow with explicit ownership and actor.
func (s *SiteService) DeleteSite(ctx context.Context, id, serverID, teamID, actorID string) error {
	return s.deleteSite(ctx, id, serverID, teamID, actorID, false)
}

// RemoveSiteRecord removes the site and its dependent records without remote cleanup.
func (s *SiteService) RemoveSiteRecord(ctx context.Context, id, serverID, teamID, actorID string) error {
	return s.deleteSite(ctx, id, serverID, teamID, actorID, true)
}

func (s *SiteService) deleteSite(ctx context.Context, id, serverID, teamID, actorID string, removeRecord bool) error {
	var site models.Site
	err := s.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the server before the site so record removal can safely check for
		// pending site uninstalls before deleting the parent and its children.
		var server servermodels.Server
		if err := tx.Select("id", "status").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND team_id = ?", serverID, teamID).First(&server).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiberutil.NotFound("Server not found")
			}
			return err
		}
		if server.Status == servertypes.ServerStatusDeleting {
			return fiberutil.Conflict("Server deletion is already in progress")
		}
		// The worker takes the same lock before reading the request. A rejected
		// enqueue rolls back the pending flag, and concurrent requests cannot
		// submit a second uninstall while this request is active.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND server_id = ? AND team_id = ?", id, serverID, teamID).First(&site).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiberutil.NotFound("Site not found")
			}
			return err
		}
		if site.UninstallationRequestedAt != nil {
			return fiberutil.Conflict("Site deletion is already in progress")
		}
		if removeRecord {
			return tx.Delete(&site).Error
		}
		if s.Queue == nil {
			return fiberutil.Internal("Site deletion queue is not configured")
		}
		// PostgreSQL timestamps retain microsecond precision; use the same value
		// in storage and the job to fence stale failure callbacks and deliveries.
		now := time.Now().UTC().Truncate(time.Microsecond)
		if err := tx.Model(&site).Updates(map[string]any{
			"uninstallation_requested_at": now,
			"uninstallation_failed_at":    nil,
		}).Error; err != nil {
			return err
		}
		site.UninstallationRequestedAt = &now
		site.UninstallationFailedAt = nil
		task, err := jobs.NewUninstallSiteTask(site.ID, serverID, stringToPtr(actorID), jobs.UninstallSiteScope{TeamID: teamID, RequestedAt: &now})
		if err != nil {
			return err
		}
		if _, err := s.Queue.EnqueueDefault(task); err != nil {
			s.LogError(err, "Failed to enqueue uninstall site task", "site_id", site.ID)
			return fiberutil.Internal("Unable to queue site deletion; please try again")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if removeRecord {
		activity.RecordEvent(ctx, "removed", actorID, &site, "Site removed from launchctl")
		s.BroadcastToTeam(teamID, "site.deleted", map[string]any{
			"site_id": site.ID, "server_id": serverID, "address": site.Address,
		})
		return nil
	}
	activity.RecordEvent(ctx, "deletion_requested", actorID, &site, "Site deletion requested")
	s.broadcastSiteUpdate(ctx, serverID, &site)
	return nil
}
