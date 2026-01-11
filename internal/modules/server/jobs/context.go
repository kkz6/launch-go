package jobs

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobContext extends the base jobs.JobContext with server-specific functionality
// This is the unified context used by both server AND site jobs
// (since all tasks ultimately run on servers via SSH)
type JobContext struct {
	DB         *gorm.DB
	Repo       contracts.Repository
	Logger     *zerolog.Logger
	WS         jobs.Broadcaster
	Dispatcher *taskrunner.Dispatcher
	TaskRepo   tasks.TaskRepository
}

// NewJobContext creates a new unified job context
func NewJobContext(
	db *gorm.DB,
	repo contracts.Repository,
	logger *zerolog.Logger,
	ws jobs.Broadcaster,
	dispatcher *taskrunner.Dispatcher,
	taskRepo tasks.TaskRepository,
) *JobContext {
	return &JobContext{
		DB:         db,
		Repo:       repo,
		Logger:     logger,
		WS:         ws,
		Dispatcher: dispatcher,
		TaskRepo:   taskRepo,
	}
}

// RunTask creates a ServerTaskDispatcher for running a task on a server
// Used by both server jobs AND site jobs (since site tasks run on servers)
//
// Usage:
//
//	result, err := j.RunTask(server, tasks.NewInstallCron(cron)).
//	    AsRoot().
//	    KeepTrack().
//	    Dispatch(ctx)
func (c *JobContext) RunTask(server *models.Server, task taskrunner.Task) *tasks.ServerTaskDispatcher {
	return tasks.NewServerTaskDispatcher(server, task, c.Dispatcher, c.TaskRepo, c.Logger)
}

// FindServer fetches a server by ID with common preloads
func (c *JobContext) FindServer(ctx context.Context, id string) (*models.Server, error) {
	var server models.Server
	if err := c.DB.First(&server, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to find server: %w", err)
	}
	return &server, nil
}

// FindServerWithPreloads fetches a server by ID with specified preloads
func (c *JobContext) FindServerWithPreloads(ctx context.Context, id string, preloads ...string) (*models.Server, error) {
	query := c.DB
	for _, preload := range preloads {
		query = query.Preload(preload)
	}

	var server models.Server
	if err := query.First(&server, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to find server: %w", err)
	}
	return &server, nil
}

// Log returns a logger for structured logging
func (c *JobContext) Log() *zerolog.Logger {
	return c.Logger
}

// Broadcast sends a websocket message to a channel
func (c *JobContext) Broadcast(channel, event string, data interface{}) {
	if c.WS != nil {
		c.WS.Broadcast(channel, event, data)
	}
}

// BroadcastToServer sends a websocket message to a server's channel
func (c *JobContext) BroadcastToServer(serverID, event string, data interface{}) {
	if c.WS != nil {
		c.WS.BroadcastToServer(serverID, event, data)
	}
}

// BroadcastToSite sends a websocket message to a site's channel
func (c *JobContext) BroadcastToSite(siteID, event string, data interface{}) {
	if c.WS != nil {
		c.WS.BroadcastToSite(siteID, event, data)
	}
}

// BroadcastToDeployment sends a websocket message to a deployment's channel
func (c *JobContext) BroadcastToDeployment(deploymentID, event string, data interface{}) {
	if c.WS != nil {
		c.WS.BroadcastToDeployment(deploymentID, event, data)
	}
}
