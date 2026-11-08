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
	var site models.Site
	err := s.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		var server servermodels.Server
		if err := tx.Where("id = ? AND team_id = ?", serverID, teamID).First(&server).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiberutil.NotFound("Server not found")
			}
			return err
		}
		if server.Status == servertypes.ServerStatusDeleting {
			return fiberutil.Conflict("Server deletion is already in progress")
		}
		if site.UninstallationRequestedAt != nil {
			return fiberutil.Conflict("Site deletion is already in progress")
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
	activity.RecordEvent(ctx, "deletion_requested", actorID, &site, "Site deletion requested")
	s.broadcastSiteUpdate(ctx, serverID, &site)
	return nil
}
