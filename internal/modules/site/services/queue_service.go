package services

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
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

// Create creates a new queue worker
func (s *QueueService) Create(ctx context.Context, siteID, serverID, userID string, req *dto.CreateQueueRequest) (*models.Queue, error) {
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
		SiteID:                site.ID,
		ServerID:              serverID,
		UserID:                userID,
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
		StopWaitSeconds:       10,
		StopSignal:            "TERM",
	}

	if req.MaxTries != nil {
		queueModel.MaxTries = req.MaxTries
	}

	if req.MaxMemory != nil {
		queueModel.MaxMemory = req.MaxMemory
	} else {
		defaultMemory := 128
		queueModel.MaxMemory = &defaultMemory
	}

	if req.NumProcs != nil {
		queueModel.NumProcs = *req.NumProcs
	} else {
		queueModel.NumProcs = 1
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

	// TODO: Dispatch queue installation job

	s.LogInfo("Queue created", "site_id", site.ID, "queue_id", queueModel.ID)

	return queueModel, nil
}

// List returns all queues for a site
func (s *QueueService) List(ctx context.Context, siteID, serverID string) ([]models.Queue, error) {
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.Repos().Queue().FindBySite(ctx, siteID)
}

// Delete deletes a queue
func (s *QueueService) Delete(ctx context.Context, queueID, siteID, serverID string) error {
	queueModel, err := s.Repos().Queue().FindByIDAndSite(ctx, queueID, siteID)
	if err != nil {
		return err
	}

	now := time.Now()
	queueModel.UninstallationRequestedAt = &now

	if err := s.Repos().Queue().Update(ctx, queueModel); err != nil {
		return err
	}

	// TODO: Dispatch queue uninstallation job

	s.LogInfo("Queue deletion requested", "queue_id", queueID)

	return nil
}

// EnableAutoRestart enables auto-restart for queue workers
func (s *QueueService) EnableAutoRestart(ctx context.Context, siteID, serverID string) error {
	return s.Repos().Site().UpdateFields(ctx, siteID, map[string]interface{}{
		"auto_restart_queue": true,
	})
}

// DisableAutoRestart disables auto-restart for queue workers
func (s *QueueService) DisableAutoRestart(ctx context.Context, siteID, serverID string) error {
	return s.Repos().Site().UpdateFields(ctx, siteID, map[string]interface{}{
		"auto_restart_queue": false,
	})
}

// SyncStatus triggers a status synchronization for all queue workers of a site
func (s *QueueService) SyncStatus(ctx context.Context, siteID, serverID, userID string) error {
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

	// Update last status check time for all queues
	now := time.Now()
	for i := range queues {
		queues[i].LastStatusCheck = &now
		if err := s.Repos().Queue().Update(ctx, &queues[i]); err != nil {
			s.LogError(err, "Failed to update queue last status check", "queue_id", queues[i].ID)
		}
	}

	// Dispatch the sync job to check supervisor status on the server
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}

	task, err := jobs.NewSyncQueuesTask(site.ID, serverID, userIDPtr)
	if err != nil {
		s.LogError(err, "Failed to create sync queues task")
		return err
	}

	if s.Queue != nil {
		if _, err := s.Queue.Enqueue(task); err != nil {
			s.LogError(err, "Failed to enqueue sync queues job")
			return err
		}
	}

	s.LogInfo("Queue sync initiated", "site_id", siteID, "queue_count", len(queues))

	return nil
}
