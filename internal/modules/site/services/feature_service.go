package services

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

var (
	ErrInvalidFeature    = fiberutil.BadRequest("Invalid feature")
	ErrFeatureNotLaravel = fiberutil.BadRequest("Features can only be managed for Laravel sites")
	ErrFeaturePending    = fiberutil.Conflict("This feature is currently being processed")
)

// taskFactoryFn creates an asynq task for a given site, server, and user
type taskFactoryFn func(siteID, serverID string, userID *string) (*asynq.Task, error)

// enableTaskFactories maps feature names to their enable task factory functions
var enableTaskFactories = map[sitetypes.LaravelFeature]taskFactoryFn{
	sitetypes.LaravelFeatureScheduler: jobs.NewEnableLaravelSchedulerTask,
	sitetypes.LaravelFeatureQueue:     jobs.NewEnableLaravelQueueTask,
	sitetypes.LaravelFeatureHorizon:   jobs.NewEnableLaravelHorizonTask,
	sitetypes.LaravelFeatureInertia:   jobs.NewEnableLaravelInertiaTask,
	sitetypes.LaravelFeatureOctane:    jobs.NewEnableLaravelOctaneTask,
	sitetypes.LaravelFeatureReverb:    jobs.NewEnableLaravelReverbTask,
}

// disableTaskFactories maps feature names to their disable task factory functions
var disableTaskFactories = map[sitetypes.LaravelFeature]taskFactoryFn{
	sitetypes.LaravelFeatureScheduler: jobs.NewDisableLaravelSchedulerTask,
	sitetypes.LaravelFeatureQueue:     jobs.NewDisableLaravelQueueTask,
	sitetypes.LaravelFeatureHorizon:   jobs.NewDisableLaravelHorizonTask,
	sitetypes.LaravelFeatureInertia:   jobs.NewDisableLaravelInertiaTask,
	sitetypes.LaravelFeatureOctane:    jobs.NewDisableLaravelOctaneTask,
	sitetypes.LaravelFeatureReverb:    jobs.NewDisableLaravelReverbTask,
}

// FeatureService handles enabling and disabling Laravel features
type FeatureService struct {
	*BaseService
	serverReader contracts.ServerReader
}

// NewFeatureService creates a new feature service
func NewFeatureService(deps *ServiceDeps) *FeatureService {
	return &FeatureService{
		BaseService: NewBaseService(deps),
	}
}

// SetServerReader sets the server reader for cross-module queries
func (s *FeatureService) SetServerReader(reader contracts.ServerReader) {
	s.serverReader = reader
}

// EnableFeatureOptions holds optional parameters for enabling a feature
type EnableFeatureOptions struct {
	DeleteQueues bool // For Horizon: delete existing queue workers before enabling
}

// EnableFeature enables a Laravel feature for a site
func (s *FeatureService) EnableFeature(ctx context.Context, siteID, serverID, teamID string, userID *string, featureName string, opts EnableFeatureOptions) error {
	feature := sitetypes.LaravelFeature(featureName)
	if !feature.IsValid() {
		return ErrInvalidFeature
	}

	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, siteID, serverID, teamID)
	if err != nil {
		return err
	}

	if !site.Type.IsLaravel() {
		return ErrFeatureNotLaravel
	}

	if site.HasPendingFeature(featureName) {
		return ErrFeaturePending
	}

	if site.HasEnabledFeature(featureName) {
		return fiberutil.Conflict(fmt.Sprintf("%s is already enabled", feature.Label()))
	}

	// Check conflicts — for Horizon with DeleteQueues, auto-remove the queue conflict
	for _, conflict := range feature.ConflictsWith() {
		if !site.HasEnabledFeature(conflict.String()) {
			continue
		}

		if feature != sitetypes.LaravelFeatureHorizon || conflict != sitetypes.LaravelFeatureQueue || !opts.DeleteQueues {
			return fiberutil.Conflict(fmt.Sprintf("Cannot enable %s while %s is enabled — disable %s first", feature.Label(), conflict.Label(), conflict.Label()))
		}

		if err := s.removeQueueFeature(ctx, site, serverID, userID); err != nil {
			return fmt.Errorf("failed to remove queue workers: %w", err)
		}
	}

	// Check Redis dependency for Horizon
	if feature == sitetypes.LaravelFeatureHorizon {
		if err := s.checkRedisInstalled(ctx, serverID); err != nil {
			return err
		}
	}

	// Add to pending features
	site.AddPendingFeature(featureName)
	if err := s.Repos().Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"pending_features": site.PendingFeatures,
	}); err != nil {
		return err
	}

	// Dispatch the enable job
	if err := s.dispatchFeatureJob(enableTaskFactories, site, serverID, userID, feature); err != nil {
		s.rollbackPendingFeature(ctx, site, featureName)
		return err
	}

	s.broadcastSiteUpdate(ctx, serverID, site)

	return nil
}

// removeQueueFeature removes the queue feature and all queue workers for a site
func (s *FeatureService) removeQueueFeature(ctx context.Context, site *models.Site, serverID string, userID *string) error {
	// Remove the queue feature from enabled_features
	site.RemoveEnabledFeature("queue")

	// Find all queue records for this site and dispatch uninstall for each
	queues, err := s.Repos().Queue().FindBySite(ctx, site.ID)
	if err != nil {
		return fmt.Errorf("failed to find queues: %w", err)
	}

	for _, q := range queues {
		task, err := jobs.NewUninstallQueueTask(site.ID, q.ID, userID)
		if err != nil {
			continue
		}
		_ = s.EnqueueTask(task)

		_ = s.Repos().Queue().Delete(ctx, q.ID)
	}

	// Update site features
	return s.Repos().Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
	})
}

// DisableFeature disables a Laravel feature for a site
func (s *FeatureService) DisableFeature(ctx context.Context, siteID, serverID, teamID string, userID *string, featureName string) error {
	feature := sitetypes.LaravelFeature(featureName)
	if !feature.IsValid() {
		return ErrInvalidFeature
	}

	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, siteID, serverID, teamID)
	if err != nil {
		return err
	}

	if !site.Type.IsLaravel() {
		return ErrFeatureNotLaravel
	}

	if site.HasPendingFeature(featureName) {
		return ErrFeaturePending
	}

	if !site.HasEnabledFeature(featureName) {
		return fiberutil.BadRequest(fmt.Sprintf("%s is not enabled", feature.Label()))
	}

	// Add to pending features
	site.AddPendingFeature(featureName)
	if err := s.Repos().Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"pending_features": site.PendingFeatures,
	}); err != nil {
		return err
	}

	// Dispatch the disable job
	if err := s.dispatchFeatureJob(disableTaskFactories, site, serverID, userID, feature); err != nil {
		s.rollbackPendingFeature(ctx, site, featureName)
		return err
	}

	s.broadcastSiteUpdate(ctx, serverID, site)

	return nil
}

// checkRedisInstalled checks if Redis is installed on the server
func (s *FeatureService) checkRedisInstalled(ctx context.Context, serverID string) error {
	if s.serverReader == nil {
		return fiberutil.BadRequest("Server reader not available")
	}

	installedServices, err := s.serverReader.FindServicesByServer(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to check installed services: %w", err)
	}

	for _, svc := range installedServices {
		if svc.Type == servertypes.ServiceTypeRedis {
			return nil
		}
	}

	return fiberutil.BadRequest("Redis must be installed on the server before enabling Horizon. Install Redis from the server's Services tab first.")
}

// broadcastSiteUpdate broadcasts a site update event
func (s *FeatureService) broadcastSiteUpdate(ctx context.Context, serverID string, site *models.Site) {
	if s.serverReader == nil {
		return
	}

	server, err := s.serverReader.FindServerByID(ctx, serverID)
	if err != nil {
		return
	}

	s.BroadcastToTeam(server.TeamID, "site.updated", map[string]any{
		"team_id":   server.TeamID,
		"server_id": server.ID,
		"site_id":   site.ID,
	})
}

// dispatchFeatureJob dispatches the appropriate job for the feature
func (s *FeatureService) dispatchFeatureJob(factories map[sitetypes.LaravelFeature]taskFactoryFn, site *models.Site, serverID string, userID *string, feature sitetypes.LaravelFeature) error {
	factory, ok := factories[feature]
	if !ok {
		return fmt.Errorf("no job registered for feature: %s", feature)
	}

	task, err := factory(site.ID, serverID, userID)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	if err := s.EnqueueTask(task); err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	return nil
}

// rollbackPendingFeature removes a feature from pending on dispatch failure
func (s *FeatureService) rollbackPendingFeature(ctx context.Context, site *models.Site, featureName string) {
	site.RemovePendingFeature(featureName)
	_ = s.Repos().Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"pending_features": site.PendingFeatures,
	})
}
