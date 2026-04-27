package services

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
)

const (
	defaultStopWaitSeconds = 10
	defaultStopSignal      = "TERM"
	defaultMaxMemory       = 128
	defaultNumProcs        = 1
)

// QueueService handles business logic for queue workers
type QueueService struct {
	*BaseService
}

// NewQueueService creates a new queue service
func NewQueueService(deps *ServiceDeps) *QueueService {
	return &QueueService{
		BaseService: NewBaseService(deps),
	}
}

// Create creates a new queue worker. Signature matches CreateDoubleNestedFunc.
func (s *QueueService) Create(ctx context.Context, siteID, serverID, teamID, userID string, req *dto.CreateQueueRequest) (dto.QueueResponse, error) {
	_ = teamID
	queue, err := s.createQueue(ctx, siteID, serverID, userID, req)
	if err != nil {
		return dto.QueueResponse{}, err
	}
	return dto.ToQueueResponse(queue), nil
}

func (s *QueueService) createQueue(ctx context.Context, siteID, serverID, userID string, req *dto.CreateQueueRequest) (*models.Queue, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	user := req.User
	if user == nil || *user == "" {
		user = &site.User
	}

	directory := req.Directory
	if directory == nil || *directory == "" {
		appDir := site.GetApplicationDirectory()
		directory = &appDir
	}

	// Convert int values to *int for the model
	restSecondsOnEmpty := req.RestSecondsOnEmpty
	maxSecondsPerJob := req.MaxSecondsPerJob
	failedJobDelaySeconds := req.FailedJobDelaySeconds

	queueModel := &models.Queue{
		Directory:             directory,
		User:                  *user,
		QueueConnection:       req.QueueConnection,
		QueueName:             req.Queue,
		RestSecondsOnEmpty:    &restSecondsOnEmpty,
		MaxSecondsPerJob:      &maxSecondsPerJob,
		FailedJobDelaySeconds: &failedJobDelaySeconds,
		RunOnMaintenance:      req.RunOnMaintenance,
		RunWithListen:         req.RunWithListen,
		AutoStart:             true,
		AutoRestart:           true,
		RedirectStderr:        true,
		StopWaitSeconds:       defaultStopWaitSeconds,
		StopSignal:            defaultStopSignal,
	}
	queueModel.SiteID = site.ID
	queueModel.TeamID = site.TeamID
	queueModel.ServerID = serverID
	queueModel.UserID = userID

	if req.MaxTries != nil {
		queueModel.MaxTries = req.MaxTries
	}

	if req.MaxMemory != nil {
		queueModel.MaxMemory = req.MaxMemory
	} else {
		mem := defaultMaxMemory
		queueModel.MaxMemory = &mem
	}

	if req.NumProcs != nil {
		queueModel.NumProcs = *req.NumProcs
	} else {
		queueModel.NumProcs = defaultNumProcs
	}

	if req.StopWaitSeconds != nil {
		queueModel.StopWaitSeconds = *req.StopWaitSeconds
	}

	if req.Environment != nil {
		queueModel.Environment = req.Environment
	}

	if err := s.Repos().Queue().Create(ctx, queueModel); err != nil {
		return nil, err
	}

	// Only install the queue on the server if the site has been deployed
	// Otherwise, the deploy callback will install pending queues after the first deployment
	if site.InstalledAt != nil {
		userIDPtr := stringToPtr(userID)

		task, err := jobs.NewInstallQueueTask(site.ID, queueModel.ID, userIDPtr)
		if err != nil {
			s.LogError(err, "Failed to create install queue task")
			return nil, err
		}

		if err := s.EnqueueTask(task); err != nil {
			s.LogError(err, "Failed to enqueue install queue job")
			return nil, err
		}
	}

	s.LogInfo("Queue created", "site_id", site.ID, "queue_id", queueModel.ID)

	return queueModel, nil
}

// List returns all queues for a site. Signature matches IndexDoubleNestedFunc.
func (s *QueueService) List(ctx context.Context, siteID, serverID, teamID string) ([]dto.QueueResponse, error) {
	_ = teamID
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}
	queues, err := s.Repos().Queue().FindBySite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.QueueResponse, len(queues))
	for i := range queues {
		out[i] = dto.ToQueueResponse(&queues[i])
	}
	return out, nil
}

// Update updates a queue worker. Signature matches UpdateDoubleNestedFunc.
func (s *QueueService) Update(ctx context.Context, queueID, siteID, serverID, teamID, userID string, req *dto.UpdateQueueRequest) (dto.QueueResponse, error) {
	_ = teamID
	queue, err := s.updateQueue(ctx, queueID, siteID, serverID, userID, req)
	if err != nil {
		return dto.QueueResponse{}, err
	}
	return dto.ToQueueResponse(queue), nil
}

func (s *QueueService) updateQueue(ctx context.Context, queueID, siteID, serverID, userID string, req *dto.UpdateQueueRequest) (*models.Queue, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	queueModel, err := s.Repos().Queue().FindByIDAndSite(ctx, queueID, siteID)
	if err != nil {
		return nil, err
	}

	if req.QueueConnection != nil {
		queueModel.QueueConnection = *req.QueueConnection
	}

	if req.Queue != nil {
		queueModel.QueueName = *req.Queue
	}

	if req.User != nil {
		queueModel.User = *req.User
	}

	if req.RestSecondsOnEmpty != nil {
		queueModel.RestSecondsOnEmpty = req.RestSecondsOnEmpty
	}

	if req.MaxSecondsPerJob != nil {
		queueModel.MaxSecondsPerJob = req.MaxSecondsPerJob
	}

	if req.FailedJobDelaySeconds != nil {
		queueModel.FailedJobDelaySeconds = req.FailedJobDelaySeconds
	}

	if req.MaxTries != nil {
		queueModel.MaxTries = req.MaxTries
	}

	if req.MaxMemory != nil {
		queueModel.MaxMemory = req.MaxMemory
	}

	if req.RunOnMaintenance != nil {
		queueModel.RunOnMaintenance = *req.RunOnMaintenance
	}

	if req.RunWithListen != nil {
		queueModel.RunWithListen = *req.RunWithListen
	}

	if req.Directory != nil {
		queueModel.Directory = req.Directory
	}

	if req.Environment != nil {
		queueModel.Environment = req.Environment
	}

	if req.NumProcs != nil {
		queueModel.NumProcs = *req.NumProcs
	}

	if req.StopWaitSeconds != nil {
		queueModel.StopWaitSeconds = *req.StopWaitSeconds
	}

	if err := s.Repos().Queue().Update(ctx, queueModel); err != nil {
		return nil, err
	}

	// Re-install the queue on the server if the site has been deployed
	if site.InstalledAt != nil {
		userIDPtr := stringToPtr(userID)

		task, err := jobs.NewInstallQueueTask(site.ID, queueModel.ID, userIDPtr)
		if err != nil {
			s.LogError(err, "Failed to create install queue task")
			return nil, err
		}

		if err := s.EnqueueTask(task); err != nil {
			s.LogError(err, "Failed to enqueue install queue job")
			return nil, err
		}
	}

	s.LogInfo("Queue updated", "site_id", site.ID, "queue_id", queueModel.ID)

	return queueModel, nil
}

// Delete deletes a queue. Signature matches DeleteDoubleNestedFunc.
func (s *QueueService) Delete(ctx context.Context, queueID, siteID, serverID, teamID, userID string) error {
	_ = teamID
	_ = userID
	queueModel, err := s.Repos().Queue().FindByIDAndSite(ctx, queueID, siteID)
	if err != nil {
		return err
	}

	now := time.Now()
	queueModel.UninstallationRequestedAt = &now

	if err := s.Repos().Queue().Update(ctx, queueModel); err != nil {
		return err
	}

	// Dispatch queue uninstallation job
	task, err := jobs.NewUninstallQueueTask(siteID, queueID, nil)
	if err != nil {
		s.LogError(err, "Failed to create uninstall queue task")
		return err
	}

	if err := s.EnqueueTask(task); err != nil {
		s.LogError(err, "Failed to enqueue uninstall queue job")
		return err
	}

	s.LogInfo("Queue deletion requested", "queue_id", queueID)

	return nil
}

// Restart restarts a single queue worker. Signature matches ActionItemDoubleNestedFunc.
func (s *QueueService) Restart(ctx context.Context, queueID, siteID, serverID, teamID, userID string) error {
	_ = teamID
	queue, err := s.Repos().Queue().FindByIDAndSite(ctx, queueID, siteID)
	if err != nil {
		return err
	}

	task, err := jobs.NewRestartQueueTask(queue.SiteID, queue.ID, stringToPtr(userID))
	if err != nil {
		s.LogError(err, "Failed to create restart queue task")
		return err
	}

	if err := s.EnqueueTask(task); err != nil {
		s.LogError(err, "Failed to enqueue restart queue job")
		return err
	}

	s.LogInfo("Queue restart initiated", "queue_id", queueID)

	return nil
}

// UpdateAutoRestart updates the auto-restart queue setting
func (s *QueueService) UpdateAutoRestart(ctx context.Context, siteID, serverID string, enabled bool) error {
	// Verify site exists and belongs to server
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	return s.Repos().Site().UpdateFields(ctx, siteID, map[string]any{
		"auto_restart_queue": enabled,
	})
}

// SyncStatus triggers a status synchronization for all queue workers of
// a site. Signature matches ActionDoubleNestedFunc.
func (s *QueueService) SyncStatus(ctx context.Context, siteID, serverID, teamID, userID string) error {
	_ = teamID
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return err
	}

	queues, err := s.Repos().Queue().FindBySite(ctx, siteID)
	if err != nil {
		return err
	}

	if len(queues) == 0 {
		s.LogInfo("No queues to sync", "site_id", siteID)
		return nil
	}

	// Dispatch the sync job to check supervisor status on the server
	task, err := jobs.NewSyncQueuesTask(site.ID, serverID, stringToPtr(userID))
	if err != nil {
		s.LogError(err, "Failed to create sync queues task")
		return err
	}

	if err := s.EnqueueTask(task); err != nil {
		s.LogError(err, "Failed to enqueue sync queues job")
		return err
	}

	s.LogInfo("Queue sync initiated", "site_id", siteID, "queue_count", len(queues))

	return nil
}
