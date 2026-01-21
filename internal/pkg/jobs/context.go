package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// Base provides common dependencies for all job contexts.
// Module-specific job contexts should embed this struct to get
// standard logging, broadcasting, and database access.
//
// Example usage:
//
//	type JobContext struct {
//	    jobs.Base
//	    Repos  *repositories.Registry
//	    // ... module-specific fields
//	}
//
//	func NewJobContext(deps jobs.BaseDeps, repos *repositories.Registry) *JobContext {
//	    return &JobContext{
//	        Base:  jobs.NewBase(deps),
//	        Repos: repos,
//	    }
//	}
type Base struct {
	db         *gorm.DB
	logger     *zerolog.Logger
	ws         broadcast.TeamBroadcaster
	dispatcher taskrunner.TaskDispatcher
	queue      *queue.Client
}

// BaseDeps holds the common dependencies for creating a Base context.
type BaseDeps struct {
	DB         *gorm.DB
	Logger     *zerolog.Logger
	WS         broadcast.TeamBroadcaster
	Dispatcher taskrunner.TaskDispatcher
	Queue      *queue.Client
}

// NewBase creates a new Base context with the provided dependencies.
func NewBase(deps BaseDeps) Base {
	return Base{
		db:         deps.DB,
		logger:     deps.Logger,
		ws:         deps.WS,
		dispatcher: deps.Dispatcher,
		queue:      deps.Queue,
	}
}

// DB returns the database connection.
func (b *Base) DB() *gorm.DB {
	return b.db
}

// Logger returns the logger.
func (b *Base) Logger() *zerolog.Logger {
	return b.logger
}

// WS returns the team broadcaster.
func (b *Base) WS() broadcast.TeamBroadcaster {
	return b.ws
}

// Dispatcher returns the task dispatcher.
func (b *Base) Dispatcher() taskrunner.TaskDispatcher {
	return b.dispatcher
}

// Queue returns the queue client.
func (b *Base) Queue() *queue.Client {
	return b.queue
}

// LogInfo logs an info message with optional key-value pairs.
// Fields should be provided as alternating key-value pairs.
//
// Example:
//
//	ctx.LogInfo("Processing job", "jobID", job.ID, "userID", userID)
func (b *Base) LogInfo(msg string, fields ...any) {
	if b.logger == nil {
		return
	}
	event := b.logger.Info()
	logWithFields(event, msg, fields...)
}

// LogError logs an error message with optional key-value pairs.
// Fields should be provided as alternating key-value pairs.
//
// Example:
//
//	ctx.LogError(err, "Failed to process job", "jobID", job.ID)
func (b *Base) LogError(err error, msg string, fields ...any) {
	if b.logger == nil {
		return
	}
	event := b.logger.Error()
	if err != nil {
		event = event.Err(err)
	}
	logWithFields(event, msg, fields...)
}

// LogWarn logs a warning message with optional key-value pairs.
// Fields should be provided as alternating key-value pairs.
//
// Example:
//
//	ctx.LogWarn("Retry limit approaching", "attempts", attempts)
func (b *Base) LogWarn(msg string, fields ...any) {
	if b.logger == nil {
		return
	}
	event := b.logger.Warn()
	logWithFields(event, msg, fields...)
}

// LogDebug logs a debug message with optional key-value pairs.
// Fields should be provided as alternating key-value pairs.
//
// Example:
//
//	ctx.LogDebug("Job payload", "payload", payload)
func (b *Base) LogDebug(msg string, fields ...any) {
	if b.logger == nil {
		return
	}
	event := b.logger.Debug()
	logWithFields(event, msg, fields...)
}

// BroadcastToTeam sends a websocket event to a team channel.
// This is a no-op if the broadcaster is nil.
//
// Example:
//
//	ctx.BroadcastToTeam(teamID, "server.updated", serverData)
func (b *Base) BroadcastToTeam(teamID, event string, data any) {
	if b.ws != nil {
		b.ws.BroadcastToTeam(teamID, event, data)
	}
}

// logWithFields applies key-value pairs to a zerolog event and sends the message.
func logWithFields(event *zerolog.Event, msg string, fields ...any) {
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}
