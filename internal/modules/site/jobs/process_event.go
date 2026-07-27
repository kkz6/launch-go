package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	siteevents "github.com/kkz6/launch-go/internal/modules/site/events"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	pkgevents "github.com/kkz6/launch-go/internal/pkg/events"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const (
	restartQueuesListenerName = "restart-site-queues"
	postDeployMaxRetries      = 3
)

var siteEventDispatcher *pkgevents.Dispatcher

// ProcessSiteEventJob fans one durable event out to all registered listeners.
type ProcessSiteEventJob struct {
	Deps       *JobDeps
	Dispatcher *pkgevents.Dispatcher
	Event      pkgevents.Event
}

func NewProcessSiteEventJob(event pkgevents.Event) pkgjobs.Handler {
	return &ProcessSiteEventJob{
		Deps:       deps,
		Dispatcher: siteEventDispatcher,
		Event:      event,
	}
}

func (j *ProcessSiteEventJob) Handle(ctx context.Context) error {
	if j.Dispatcher == nil {
		return fmt.Errorf("site event dispatcher is not configured")
	}
	return j.Dispatcher.Dispatch(ctx, j.Event)
}

func (j *ProcessSiteEventJob) Failed(_ context.Context, err error) {
	j.Deps.Logger.Error().
		Err(err).
		Str("event_id", j.Event.ID).
		Str("event_name", j.Event.Name).
		Msg("site event dispatch failed")
}

// restartQueuesOnDeploymentSucceeded is deliberately a fan-out listener: it
// enqueues the independently retryable server action rather than performing
// SSH inside the event worker. The event/listener task ID makes retries and
// callback recovery idempotent.
type restartQueuesOnDeploymentSucceeded struct {
	sites  siteEventSiteReader
	queue  siteEventTaskEnqueuer
	logger *zerolog.Logger
}

type siteEventSiteReader interface {
	FindByID(ctx context.Context, id string) (*models.Site, error)
}

type siteEventTaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

func newRestartQueuesOnDeploymentSucceeded(deps *JobDeps) pkgevents.Listener {
	return &restartQueuesOnDeploymentSucceeded{
		sites:  deps.Repos.Site(),
		queue:  deps.Queue,
		logger: deps.Logger,
	}
}

func (l *restartQueuesOnDeploymentSucceeded) Name() string {
	return restartQueuesListenerName
}

func (l *restartQueuesOnDeploymentSucceeded) Handle(ctx context.Context, event pkgevents.Event) error {
	payload, err := pkgevents.Decode[siteevents.DeploymentSucceeded](event)
	if err != nil {
		return err
	}

	site, err := l.sites.FindByID(ctx, payload.SiteID)
	if err != nil {
		return fmt.Errorf("find site: %w", err)
	}
	if !site.AutoRestartQueue {
		return nil
	}
	if l.queue == nil {
		return fmt.Errorf("queue client is not configured")
	}

	taskID := pkgjobs.Dedup("site-event", event.ID, l.Name())
	task, err := NewRestartAllSiteQueuesTask(
		payload.SiteID,
		nil,
		asynq.TaskID(taskID),
		asynq.MaxRetry(postDeployMaxRetries),
	)
	if err != nil {
		return fmt.Errorf("create restart queues task: %w", err)
	}
	if _, err := l.queue.Enqueue(task); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
			return nil
		}
		return fmt.Errorf("enqueue restart queues task: %w", err)
	}

	if l.logger != nil {
		l.logger.Info().
			Str("event_id", event.ID).
			Str("deployment_id", payload.DeploymentID).
			Str("site_id", payload.SiteID).
			Msg("queued post-deployment queue restart")
	}
	return nil
}

func configureSiteEventDispatcher(jobDeps *JobDeps) *pkgevents.Dispatcher {
	dispatcher := pkgevents.NewDispatcher()
	dispatcher.MustSubscribe(
		siteevents.DeploymentSucceededName,
		newRestartQueuesOnDeploymentSucceeded(jobDeps),
	)
	return dispatcher
}
