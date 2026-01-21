package activity

import (
	"context"

	"gorm.io/gorm"
)

var defaultDB *gorm.DB

func SetDB(db *gorm.DB) {
	defaultDB = db
}

func Activity() *Logger {
	if defaultDB == nil {
		panic("activity: database not initialized, call activity.SetDB() first")
	}
	return New(defaultDB)
}

func Log(description string) (*ActivityLog, error) {
	return Activity().Log(description)
}

func LogWithContext(ctx context.Context, description string) (*ActivityLog, error) {
	return Activity().WithContext(ctx).Log(description)
}

type HasActivities interface {
	Subject
}

func GetActivitiesFor(ctx context.Context, subject HasActivities, limit int) ([]ActivityLog, error) {
	if defaultDB == nil {
		panic("activity: database not initialized, call activity.SetDB() first")
	}
	repo := NewRepository(defaultDB)
	return repo.Query().
		WithSubject(getTypeName(subject), subject.GetID()).
		Limit(limit).
		OrderByLatest().
		Get(ctx)
}

func GetActivitiesByUser(ctx context.Context, userID string, limit int) ([]ActivityLog, error) {
	if defaultDB == nil {
		panic("activity: database not initialized, call activity.SetDB() first")
	}
	repo := NewRepository(defaultDB)
	return repo.Query().
		WithCauser("User", userID).
		Limit(limit).
		OrderByLatest().
		Get(ctx)
}

// -------------------------------------------------------------------------
// Shorthand logging functions for common CRUD operations
// -------------------------------------------------------------------------

// LogCreated logs a "created" activity for a subject.
// It uses the subject's type name (lowercased) as the log name.
// Example: LogCreated(ctx, db, userID, server, "Server was created")
func LogCreated(ctx context.Context, db *gorm.DB, userID string, subject Subject, description string) (*ActivityLog, error) {
	return New(db).
		WithContext(ctx).
		UseLog(getLogName(subject)).
		CausedByUser(userID).
		On(subject).
		WithEvent("created").
		Log(description)
}

// LogUpdated logs an "updated" activity for a subject.
// It uses the subject's type name (lowercased) as the log name.
// Example: LogUpdated(ctx, db, userID, server, "Server was updated")
func LogUpdated(ctx context.Context, db *gorm.DB, userID string, subject Subject, description string) (*ActivityLog, error) {
	return New(db).
		WithContext(ctx).
		UseLog(getLogName(subject)).
		CausedByUser(userID).
		On(subject).
		WithEvent("updated").
		Log(description)
}

// LogDeleted logs a "deleted" activity for a subject.
// It uses the subject's type name (lowercased) as the log name.
// Example: LogDeleted(ctx, db, userID, server, "Server was deleted")
func LogDeleted(ctx context.Context, db *gorm.DB, userID string, subject Subject, description string) (*ActivityLog, error) {
	return New(db).
		WithContext(ctx).
		UseLog(getLogName(subject)).
		CausedByUser(userID).
		On(subject).
		WithEvent("deleted").
		Log(description)
}

// LogEvent logs a custom event for a subject with an optional user.
// Useful for events like "archived", "deployed", "installed", etc.
// Pass empty string for userID if no user caused the event.
// Example: LogEvent(ctx, db, "archived", userID, server, "Server was archived")
func LogEvent(ctx context.Context, db *gorm.DB, event string, userID string, subject Subject, description string) (*ActivityLog, error) {
	logger := New(db).
		WithContext(ctx).
		UseLog(getLogName(subject)).
		On(subject).
		WithEvent(event)

	if userID != "" {
		logger.CausedByUser(userID)
	}

	return logger.Log(description)
}

// LogEventWithProps logs a custom event with additional properties.
// Useful when you need to store extra data with the activity.
// Example: LogEventWithProps(ctx, db, "deployed", userID, site, "Deployed to production", map[string]any{"version": "1.0.0"})
func LogEventWithProps(ctx context.Context, db *gorm.DB, event string, userID string, subject Subject, description string, props map[string]any) (*ActivityLog, error) {
	logger := New(db).
		WithContext(ctx).
		UseLog(getLogName(subject)).
		On(subject).
		WithEvent(event).
		WithProperties(props)

	if userID != "" {
		logger.CausedByUser(userID)
	}

	return logger.Log(description)
}

// LogWithLog logs an activity with a custom log name (instead of deriving from subject type).
// Useful when you want to group activities under a different category.
// Example: LogWithLog(ctx, db, "auth", "created", userID, team, "Team was created")
func LogWithLog(ctx context.Context, db *gorm.DB, logName, event, userID string, subject Subject, description string) (*ActivityLog, error) {
	logger := New(db).
		WithContext(ctx).
		UseLog(logName).
		On(subject).
		WithEvent(event)

	if userID != "" {
		logger.CausedByUser(userID)
	}

	return logger.Log(description)
}

// -------------------------------------------------------------------------
// Pointer-friendly variants for job contexts where userID is *string
// -------------------------------------------------------------------------

// LogEventPtr is like LogEvent but accepts *string for userID.
// Useful in job handlers where Payload.UserID is typically *string.
// Example: LogEventPtr(ctx, db, "installed", payload.UserID, database, "Database was installed")
func LogEventPtr(ctx context.Context, db *gorm.DB, event string, userID *string, subject Subject, description string) (*ActivityLog, error) {
	uid := ""
	if userID != nil {
		uid = *userID
	}
	return LogEvent(ctx, db, event, uid, subject, description)
}

// LogWithLogPtr is like LogWithLog but accepts *string for userID.
// Example: LogWithLogPtr(ctx, db, "server", "installed", payload.UserID, cron, "Cron was installed")
func LogWithLogPtr(ctx context.Context, db *gorm.DB, logName, event string, userID *string, subject Subject, description string) (*ActivityLog, error) {
	uid := ""
	if userID != nil {
		uid = *userID
	}
	return LogWithLog(ctx, db, logName, event, uid, subject, description)
}

// LogWithLogAndPropsPtr logs an activity with a custom log name and properties.
// Accepts *string for userID (common in job payloads).
// Example: LogWithLogAndPropsPtr(ctx, db, "server", "audit_completed", payload.UserID, server, "Audit done", props)
func LogWithLogAndPropsPtr(ctx context.Context, db *gorm.DB, logName, event string, userID *string, subject Subject, description string, props map[string]any) (*ActivityLog, error) {
	uid := ""
	if userID != nil {
		uid = *userID
	}

	logger := New(db).
		WithContext(ctx).
		UseLog(logName).
		On(subject).
		WithEvent(event).
		WithProperties(props)

	if uid != "" {
		logger.CausedByUser(uid)
	}

	return logger.Log(description)
}
