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
