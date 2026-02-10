package activity

import (
	"context"

	"gorm.io/gorm"
)

var defaultDB *gorm.DB

func SetDB(db *gorm.DB) {
	defaultDB = db
}

// IsInitialized returns true if the activity logger's DB has been set.
func IsInitialized() bool {
	return defaultDB != nil
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

// -------------------------------------------------------------------------
// Service-friendly variants that use the global DB
// These functions allow services to log activities without accessing DB directly
// -------------------------------------------------------------------------

// RecordCreated logs a "created" activity using the global DB.
// Example: RecordCreated(ctx, userID, server, "Server was created")
func RecordCreated(ctx context.Context, userID string, subject Subject, description string) {
	if defaultDB == nil {
		return
	}
	_, _ = LogCreated(ctx, defaultDB, userID, subject, description)
}

// RecordUpdated logs an "updated" activity using the global DB.
// Example: RecordUpdated(ctx, userID, server, "Server was updated")
func RecordUpdated(ctx context.Context, userID string, subject Subject, description string) {
	if defaultDB == nil {
		return
	}
	_, _ = LogUpdated(ctx, defaultDB, userID, subject, description)
}

// RecordDeleted logs a "deleted" activity using the global DB.
// Example: RecordDeleted(ctx, userID, server, "Server was deleted")
func RecordDeleted(ctx context.Context, userID string, subject Subject, description string) {
	if defaultDB == nil {
		return
	}
	_, _ = LogDeleted(ctx, defaultDB, userID, subject, description)
}

// RecordEvent logs a custom event using the global DB.
// Example: RecordEvent(ctx, "archived", userID, server, "Server was archived")
func RecordEvent(ctx context.Context, event, userID string, subject Subject, description string) {
	if defaultDB == nil {
		return
	}
	_, _ = LogEvent(ctx, defaultDB, event, userID, subject, description)
}

// RecordEventWithProps logs a custom event with properties using the global DB.
// Example: RecordEventWithProps(ctx, "deployed", userID, site, "Deployed", map[string]any{"version": "1.0.0"})
func RecordEventWithProps(ctx context.Context, event, userID string, subject Subject, description string, props map[string]any) {
	if defaultDB == nil {
		return
	}
	_, _ = LogEventWithProps(ctx, defaultDB, event, userID, subject, description, props)
}

// RecordWithLog logs an activity with a custom log name using the global DB.
// Example: RecordWithLog(ctx, "auth", "created", userID, team, "Team was created")
func RecordWithLog(ctx context.Context, logName, event, userID string, subject Subject, description string) {
	if defaultDB == nil {
		return
	}
	_, _ = LogWithLog(ctx, defaultDB, logName, event, userID, subject, description)
}

// RecordEventPtr logs a custom event using the global DB (pointer-friendly).
// Example: RecordEventPtr(ctx, "installed", payload.UserID, database, "Database was installed")
func RecordEventPtr(ctx context.Context, event string, userID *string, subject Subject, description string) {
	if defaultDB == nil {
		return
	}
	_, _ = LogEventPtr(ctx, defaultDB, event, userID, subject, description)
}

// RecordWithLogPtr logs with a custom log name using the global DB (pointer-friendly).
// Example: RecordWithLogPtr(ctx, "server", "installed", payload.UserID, cron, "Cron was installed")
func RecordWithLogPtr(ctx context.Context, logName, event string, userID *string, subject Subject, description string) {
	if defaultDB == nil {
		return
	}
	_, _ = LogWithLogPtr(ctx, defaultDB, logName, event, userID, subject, description)
}

// RecordWithLogAndPropsPtr logs with a custom log name and properties using the global DB.
// Example: RecordWithLogAndPropsPtr(ctx, "server", "audit_completed", payload.UserID, server, "Audit done", props)
func RecordWithLogAndPropsPtr(ctx context.Context, logName, event string, userID *string, subject Subject, description string, props map[string]any) {
	if defaultDB == nil {
		return
	}
	_, _ = LogWithLogAndPropsPtr(ctx, defaultDB, logName, event, userID, subject, description, props)
}
