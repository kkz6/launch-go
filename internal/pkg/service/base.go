package service

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/activity"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// Dependencies holds common dependencies needed by all services.
// Use this struct to pass dependencies consistently across modules.
//
// Example usage in a module:
//
//	type ServiceDeps struct {
//	    service.Dependencies
//	    Repos *repositories.Registry
//	    // ... module-specific fields
//	}
type Dependencies struct {
	DB          *gorm.DB
	Logger      *zerolog.Logger
	Queue       *queue.Client
	Broadcaster broadcast.ModelBroadcaster
	Dispatcher  *taskrunner.Dispatcher
}

// NewDependencies creates a new Dependencies instance with all common dependencies.
func NewDependencies(db *gorm.DB, log *zerolog.Logger, q *queue.Client, ws broadcast.ModelBroadcaster, dispatcher *taskrunner.Dispatcher) Dependencies {
	return Dependencies{
		DB:          db,
		Logger:      log,
		Queue:       q,
		Broadcaster: ws,
		Dispatcher:  dispatcher,
	}
}

// Base provides common service dependencies.
// Embed this in your service to get common functionality.
//
// Usage:
//
//	type DatabaseService struct {
//	    service.Base
//	    repo *repositories.Repository
//	}
type Base struct {
	broadcast.ModelMixin
	db     *gorm.DB
	Queue  *queue.Client
	Logger *zerolog.Logger
}

// NewBase creates a new Base service
func NewBase(q *queue.Client, ws broadcast.ModelBroadcaster, log *zerolog.Logger) Base {
	b := Base{
		Queue:  q,
		Logger: log,
	}
	b.SetModelBroadcaster(ws)
	return b
}

// NewBaseWithDB creates a new Base service with database access
func NewBaseWithDB(db *gorm.DB, q *queue.Client, ws broadcast.ModelBroadcaster, log *zerolog.Logger) Base {
	b := Base{
		db:     db,
		Queue:  q,
		Logger: log,
	}
	b.SetModelBroadcaster(ws)
	return b
}

// NewBaseFromDeps creates a new Base service from Dependencies
func NewBaseFromDeps(deps Dependencies) Base {
	b := Base{
		db:     deps.DB,
		Queue:  deps.Queue,
		Logger: deps.Logger,
	}
	b.SetModelBroadcaster(deps.Broadcaster)
	return b
}

// EnqueueTask enqueues a task to the default queue
func (s *Base) EnqueueTask(task *asynq.Task) error {
	if s.Queue == nil {
		return nil // Silently succeed in test mode
	}

	_, err := s.Queue.EnqueueDefault(task)
	return err
}

// EnqueueTaskWithOptions enqueues a task with custom options
func (s *Base) EnqueueTaskWithOptions(task *asynq.Task, opts ...asynq.Option) error {
	if s.Queue == nil {
		return nil
	}

	_, err := s.Queue.Enqueue(task, opts...)
	return err
}

// TaskFactory is a function that creates an asynq task
type TaskFactory func() (*asynq.Task, error)

// DispatchTask creates and enqueues a task using the provided factory.
// It handles all error logging internally and optionally logs success.
//
// Usage:
//
//	s.DispatchTask("InstallBackup", func() (*asynq.Task, error) {
//	    return jobs.NewInstallBackupTask(serverID, backupID, nil)
//	}, "server_id", serverID, "backup_id", backupID)
func (s *Base) DispatchTask(name string, factory TaskFactory, logFields ...interface{}) {
	if s.Queue == nil {
		s.LogWarn("Queue not configured, skipping task", "task", name)
		return
	}

	task, err := factory()
	if err != nil {
		s.LogError(err, "Failed to create task", "task", name)
		return
	}

	if err := s.EnqueueTask(task); err != nil {
		s.LogError(err, "Failed to enqueue task", "task", name)
		return
	}

	if len(logFields) > 0 {
		fields := append([]interface{}{"task", name}, logFields...)
		s.LogInfo("Task enqueued", fields...)
	}
}

// LogError logs an error with context
func (s *Base) LogError(err error, msg string, fields ...interface{}) {
	logger.Error(s.Logger, err, msg, fields...)
}

// LogWarn logs a warning with context
func (s *Base) LogWarn(msg string, fields ...interface{}) {
	logger.Warn(s.Logger, msg, fields...)
}

// LogInfo logs an info message with context
func (s *Base) LogInfo(msg string, fields ...interface{}) {
	logger.Info(s.Logger, msg, fields...)
}

// HasQueue returns true if a queue client is configured
func (s *Base) HasQueue() bool {
	return s.Queue != nil
}

// HasWebsocket returns true if a websocket hub is configured
func (s *Base) HasWebsocket() bool {
	return s.HasModelBroadcaster()
}

// -----------------------------------------------------------------------------
// Activity Logging Helpers
// -----------------------------------------------------------------------------

// LogActivity logs an activity for a model without a causer.
// This is a convenience method for common activity logging patterns.
//
// Usage:
//
//	s.LogActivity(ctx, "server", "created", "Server was created", server)
func (s *Base) LogActivity(ctx context.Context, logName, event, message string, model activity.Subject) {
	if s.db == nil {
		return
	}
	activity.New(s.db).
		WithContext(ctx).
		UseLog(logName).
		On(model).
		WithEvent(event).
		Log(message)
}

// LogActivityByUser logs an activity for a model caused by a user.
// This is a convenience method for common activity logging patterns.
//
// Usage:
//
//	s.LogActivityByUser(ctx, userID, "server", "created", "Server was created", server)
func (s *Base) LogActivityByUser(ctx context.Context, userID, logName, event, message string, model activity.Subject) {
	if s.db == nil {
		return
	}
	activity.New(s.db).
		WithContext(ctx).
		UseLog(logName).
		CausedByUser(userID).
		On(model).
		WithEvent(event).
		Log(message)
}

// LogActivityWithProps logs an activity with additional properties.
// This is useful when you need to add extra context to the activity log.
//
// Usage:
//
//	s.LogActivityWithProps(ctx, "server", "failed", "Server provisioning failed", server, map[string]any{
//	    "error": err.Error(),
//	    "step": "install_php",
//	})
func (s *Base) LogActivityWithProps(ctx context.Context, logName, event, message string, model activity.Subject, props map[string]any) {
	if s.db == nil {
		return
	}
	activity.New(s.db).
		WithContext(ctx).
		UseLog(logName).
		On(model).
		WithEvent(event).
		WithProperties(props).
		Log(message)
}

// LogActivityByUserWithProps logs an activity caused by a user with additional properties.
//
// Usage:
//
//	s.LogActivityByUserWithProps(ctx, userID, "server", "updated", "Server settings changed", server, map[string]any{
//	    "changes": changedFields,
//	})
func (s *Base) LogActivityByUserWithProps(ctx context.Context, userID, logName, event, message string, model activity.Subject, props map[string]any) {
	if s.db == nil {
		return
	}
	activity.New(s.db).
		WithContext(ctx).
		UseLog(logName).
		CausedByUser(userID).
		On(model).
		WithEvent(event).
		WithProperties(props).
		Log(message)
}

// ActivityLogger returns an activity logger configured with the service's database.
// Use this for more complex activity logging scenarios that require the full fluent API.
//
// Usage:
//
//	s.ActivityLogger().
//	    WithContext(ctx).
//	    UseLog("server").
//	    CausedByUser(userID).
//	    On(server).
//	    WithEvent("created").
//	    WithProperty("source", "api").
//	    Log("Server was created")
func (s *Base) ActivityLogger() *activity.Logger {
	if s.db == nil {
		return nil
	}
	return activity.New(s.db)
}

// HasDB returns true if a database connection is configured
func (s *Base) HasDB() bool {
	return s.db != nil
}

// DB returns the underlying database connection.
// This is useful for services that need direct database access.
func (s *Base) DB() *gorm.DB {
	return s.db
}

// -----------------------------------------------------------------------------
// Transaction + Activity Logging Helpers
// -----------------------------------------------------------------------------

// WithTransaction executes fn within a database transaction.
// If fn returns an error, the transaction is rolled back.
//
// Usage:
//
//	err := s.WithTransaction(ctx, func(tx *gorm.DB) error {
//	    if err := tx.Create(&server).Error; err != nil {
//	        return err
//	    }
//	    if err := tx.Create(&site).Error; err != nil {
//	        return err
//	    }
//	    return nil
//	})
func (s *Base) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if s.db == nil {
		return fn(nil)
	}

	return s.db.WithContext(ctx).Transaction(fn)
}

// WithTransactionAndLog executes fn within a transaction and logs activity on success.
// The activity is logged using the provided parameters.
//
// Usage:
//
//	err := s.WithTransactionAndLog(
//	    ctx,
//	    userID,
//	    server,
//	    "created",
//	    "Server was created",
//	    func(tx *gorm.DB) error {
//	        return tx.Create(&server).Error
//	    },
//	)
func (s *Base) WithTransactionAndLog(
	ctx context.Context,
	userID string,
	subject activity.Subject,
	event string,
	description string,
	fn func(tx *gorm.DB) error,
) error {
	return s.WithTransactionAndLogProps(ctx, userID, subject, event, description, nil, fn)
}

// WithTransactionAndLogProps is like WithTransactionAndLog but with additional properties.
//
// Usage:
//
//	err := s.WithTransactionAndLogProps(
//	    ctx,
//	    userID,
//	    server,
//	    "updated",
//	    "Server settings changed",
//	    map[string]any{"changes": changedFields},
//	    func(tx *gorm.DB) error {
//	        return tx.Save(&server).Error
//	    },
//	)
func (s *Base) WithTransactionAndLogProps(
	ctx context.Context,
	userID string,
	subject activity.Subject,
	event string,
	description string,
	props map[string]any,
	fn func(tx *gorm.DB) error,
) error {
	if s.db == nil {
		return fn(nil)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := fn(tx); err != nil {
			return err
		}

		logger := activity.New(tx).
			WithContext(ctx).
			On(subject).
			WithEvent(event)

		if userID != "" {
			logger = logger.CausedByUser(userID)
		}

		if len(props) > 0 {
			logger = logger.WithProperties(props)
		}

		_, err := logger.Log(description)

		return err
	})
}
