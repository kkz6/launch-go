package activity

import (
	"context"

	"gorm.io/gorm"
)

// ActivityMixin provides consistent activity logging for services.
// Embed this in your service to get activity logging methods with a fixed log name.
//
// Usage:
//
//	type ServerService struct {
//	    activity.ActivityMixin
//	    repo *repositories.Repository
//	}
//
//	func NewServerService(db *gorm.DB, repo *repositories.Repository) *ServerService {
//	    return &ServerService{
//	        ActivityMixin: activity.NewActivityMixin(db, "server"),
//	        repo:          repo,
//	    }
//	}
//
//	func (s *ServerService) Create(ctx context.Context, server *models.Server, userID *string) error {
//	    // ... create logic
//	    s.LogActivity(ctx, server, "created", "Server was created", userID)
//	    return nil
//	}
type ActivityMixin struct {
	db      *gorm.DB
	logName string
}

// NewActivityMixin creates a new ActivityMixin with the specified database and log name.
// The logName is used as the log category for all activities logged through this mixin.
func NewActivityMixin(db *gorm.DB, logName string) ActivityMixin {
	return ActivityMixin{db: db, logName: logName}
}

// LogActivity logs an activity for a model with an optional user as the causer.
// This is the primary method for logging activities through the mixin.
//
// Parameters:
//   - ctx: The context for the database operation
//   - model: The subject of the activity (must implement Subject interface)
//   - event: The event type (e.g., "created", "updated", "deleted", "installed")
//   - description: Human-readable description of what happened
//   - userID: Optional pointer to the user ID who caused the activity (nil for system actions)
func (m *ActivityMixin) LogActivity(ctx context.Context, model Subject, event, description string, userID *string) {
	if m.db == nil {
		return
	}

	logger := New(m.db).
		WithContext(ctx).
		UseLog(m.logName).
		On(model).
		WithEvent(event)

	if userID != nil {
		logger.CausedByUser(*userID)
	}

	logger.Log(description)
}

// LogActivityWithProps logs an activity with additional properties.
// Use this when you need to store extra context with the activity log entry.
//
// Parameters:
//   - ctx: The context for the database operation
//   - model: The subject of the activity
//   - event: The event type
//   - description: Human-readable description
//   - userID: Optional pointer to the user ID who caused the activity
//   - props: Additional properties to store with the activity
func (m *ActivityMixin) LogActivityWithProps(ctx context.Context, model Subject, event, description string, userID *string, props map[string]any) {
	if m.db == nil {
		return
	}

	logger := New(m.db).
		WithContext(ctx).
		UseLog(m.logName).
		On(model).
		WithEvent(event).
		WithProperties(props)

	if userID != nil {
		logger.CausedByUser(*userID)
	}

	logger.Log(description)
}

// LogActivityByUser logs an activity caused by a specific user.
// This is a convenience method when you always have a user ID (not optional).
func (m *ActivityMixin) LogActivityByUser(ctx context.Context, model Subject, event, description, userID string) {
	if m.db == nil {
		return
	}

	New(m.db).
		WithContext(ctx).
		UseLog(m.logName).
		CausedByUser(userID).
		On(model).
		WithEvent(event).
		Log(description)
}

// LogActivityByUserWithProps logs a user-caused activity with additional properties.
func (m *ActivityMixin) LogActivityByUserWithProps(ctx context.Context, model Subject, event, description, userID string, props map[string]any) {
	if m.db == nil {
		return
	}

	New(m.db).
		WithContext(ctx).
		UseLog(m.logName).
		CausedByUser(userID).
		On(model).
		WithEvent(event).
		WithProperties(props).
		Log(description)
}

// LogSystemActivity logs an activity without a causer (system-initiated).
// Use this for automated actions, background jobs, or scheduled tasks.
func (m *ActivityMixin) LogSystemActivity(ctx context.Context, model Subject, event, description string) {
	if m.db == nil {
		return
	}

	New(m.db).
		WithContext(ctx).
		UseLog(m.logName).
		On(model).
		WithEvent(event).
		Log(description)
}

// LogSystemActivityWithProps logs a system-initiated activity with additional properties.
func (m *ActivityMixin) LogSystemActivityWithProps(ctx context.Context, model Subject, event, description string, props map[string]any) {
	if m.db == nil {
		return
	}

	New(m.db).
		WithContext(ctx).
		UseLog(m.logName).
		On(model).
		WithEvent(event).
		WithProperties(props).
		Log(description)
}

// ActivityLogger returns a pre-configured activity logger using the mixin's log name.
// Use this for more complex logging scenarios that require the full fluent API.
func (m *ActivityMixin) ActivityLogger() *Logger {
	if m.db == nil {
		return nil
	}

	return New(m.db).UseLog(m.logName)
}

// SetActivityDB updates the database connection for the mixin.
// This is useful when the database connection is set after initialization.
func (m *ActivityMixin) SetActivityDB(db *gorm.DB) {
	m.db = db
}

// SetActivityLogName updates the log name for the mixin.
func (m *ActivityMixin) SetActivityLogName(logName string) {
	m.logName = logName
}

// HasActivityDB returns true if the mixin has a database connection configured.
func (m *ActivityMixin) HasActivityDB() bool {
	return m.db != nil
}
