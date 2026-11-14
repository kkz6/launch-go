package services

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RefreshRuntime retries runtime activation without switching code or rerunning hooks.
func (s *DeploymentService) RefreshRuntime(ctx context.Context, siteID, serverID, teamID, userID string) error {
	site, err := s.scopedSite(ctx, siteID, serverID, teamID)
	if err != nil {
		return err
	}
	if !site.ZeroDowntimeDeployment || !(site.Type.IsPHP() || site.Type == sitetypes.SiteTypeGeneric) {
		return fiberutil.BadRequest("PHP runtime refresh requires a PHP site with zero downtime deployment enabled")
	}
	if site.PhpVersion == nil || !site.PhpVersion.IsValid() {
		return fiberutil.BadRequest("Select a valid PHP version before refreshing the runtime")
	}
	if err := ensureSiteConfigurationIdle(site); err != nil {
		return err
	}
	active, err := s.Repos().Deployment().FindActiveBySite(ctx, site.ID)
	if err != nil {
		return err
	}
	if active != nil {
		return ErrPendingDeployment
	}
	task, err := jobs.NewRefreshRuntimeTask(site.ID, site.ServerID, site.TeamID,
		asynq.Unique(2*time.Minute), asynq.Timeout(90*time.Second))
	if err != nil {
		return err
	}
	if s.Queue == nil {
		return fiberutil.Internal("PHP runtime refresh could not be queued. Please try again.")
	}
	if _, err := s.Queue.EnqueueDefault(task); err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		s.LogError(err, "Failed to queue PHP runtime refresh", "site_id", site.ID)
		return fiberutil.Internal("PHP runtime refresh could not be queued. Please try again.")
	}
	return nil
}
