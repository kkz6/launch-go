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
