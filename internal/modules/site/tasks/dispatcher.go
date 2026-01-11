package tasks

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// SiteTaskDispatcher wraps PendingTask with site-specific behavior
// It provides a fluent API for configuring and dispatching tasks on a site
type SiteTaskDispatcher struct {
	site        *models.Site
	server      *servermodels.Server
	pendingTask *taskrunner.PendingTask
	dispatcher  *taskrunner.Dispatcher
	logger      *zerolog.Logger

	// Options
	throwOnFail bool
}

// NewSiteTaskDispatcher creates a new SiteTaskDispatcher for the given site and task
func NewSiteTaskDispatcher(
	site *models.Site,
	server *servermodels.Server,
	task taskrunner.Task,
	dispatcher *taskrunner.Dispatcher,
	logger *zerolog.Logger,
) *SiteTaskDispatcher {
	return &SiteTaskDispatcher{
		site:        site,
		server:      server,
		pendingTask: taskrunner.NewPendingTask(task),
		dispatcher:  dispatcher,
		logger:      logger,
	}
}

// AsRoot configures the task to run as root user on the server
func (d *SiteTaskDispatcher) AsRoot() *SiteTaskDispatcher {
	conn := d.server.ConnectionAsRoot()
	d.pendingTask.OnConnection(conn)
	return d
}

// AsUser configures the task to run as the site's user
func (d *SiteTaskDispatcher) AsUser(username ...string) *SiteTaskDispatcher {
	var user string
	if len(username) > 0 && username[0] != "" {
		user = username[0]
	} else {
		// Default to the site's user
		user = d.site.User
	}
	conn := d.server.ConnectionAsUser(user)
	d.pendingTask.OnConnection(conn)
	return d
}

// AsSiteUser configures the task to run as the site's owner
func (d *SiteTaskDispatcher) AsSiteUser() *SiteTaskDispatcher {
	return d.AsUser(d.site.User)
}

// InBackground sets the task to run in background mode
func (d *SiteTaskDispatcher) InBackground() *SiteTaskDispatcher {
	d.pendingTask.InBackground()
	return d
}

// InForeground sets the task to run in foreground mode
func (d *SiteTaskDispatcher) InForeground() *SiteTaskDispatcher {
	d.pendingTask.InForeground()
	return d
}

// Throw configures the dispatcher to return an error if the task fails
func (d *SiteTaskDispatcher) Throw() *SiteTaskDispatcher {
	d.throwOnFail = true
	return d
}

// As sets a custom task ID
func (d *SiteTaskDispatcher) As(id string) *SiteTaskDispatcher {
	d.pendingTask.As(id)
	return d
}

// OnOutput sets a callback for output streaming
func (d *SiteTaskDispatcher) OnOutput(callback func(output string)) *SiteTaskDispatcher {
	d.pendingTask.OnOutput(callback)
	return d
}

// Dispatch executes the task and returns the result
func (d *SiteTaskDispatcher) Dispatch(ctx context.Context) (*SiteDispatchResult, error) {
	// Validate connection is set
	if d.pendingTask.GetConnection() == nil {
		return nil, fmt.Errorf("no connection selected: call AsRoot(), AsUser(), or AsSiteUser() first")
	}

	// Generate task ID if not set
	if d.pendingTask.GetID() == "" {
		d.pendingTask.As("site-task-" + utils.NewULID())
	}

	// Execute the task
	result, err := d.dispatcher.Run(ctx, d.pendingTask)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch task: %w", err)
	}

	// Check if we should throw on failure
	if d.throwOnFail && !result.IsSuccessful() {
		return nil, fmt.Errorf("task '%s' failed with exit code %d: %s",
			d.pendingTask.Task.Name(), result.ExitCode, result.Output)
	}

	return &SiteDispatchResult{
		TaskResult: result,
	}, nil
}

// SiteDispatchResult contains the result of site task dispatch
type SiteDispatchResult struct {
	TaskResult *taskrunner.TaskResult
}

// IsSuccessful returns true if the task completed successfully
func (r *SiteDispatchResult) IsSuccessful() bool {
	if r.TaskResult != nil {
		return r.TaskResult.IsSuccessful()
	}
	return false
}
