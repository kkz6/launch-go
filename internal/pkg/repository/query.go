package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// Query provides a fluent query builder with OrFail support
type Query[T any] struct {
	db        *gorm.DB
	ctx       context.Context
	modelName string
}

// NewQuery creates a new Query builder
func NewQuery[T any](db *gorm.DB, ctx context.Context) *Query[T] {
	var entity T
	q := &Query[T]{
		db:        db.WithContext(ctx),
		ctx:       ctx,
		modelName: getTypeName(entity),
	}
	return q
}

// WithModel sets a custom model name for error messages
func (q *Query[T]) WithModel(name string) *Query[T] {
	q.modelName = name
	return q
}

// Preload adds preload relations
func (q *Query[T]) Preload(relations ...string) *Query[T] {
	for _, rel := range relations {
		q.db = q.db.Preload(rel)
	}
	return q
}

// Where adds a where clause
func (q *Query[T]) Where(query interface{}, args ...interface{}) *Query[T] {
	q.db = q.db.Where(query, args...)
	return q
}

// FindByID finds a record by ID
func (q *Query[T]) FindByID(id string) *Query[T] {
	q.db = q.db.Where("id = ?", id)
	return q
}

// FindByIDAndServer finds a record by ID and server ID
func (q *Query[T]) FindByIDAndServer(id, serverID string) *Query[T] {
	q.db = q.db.Where("id = ? AND server_id = ?", id, serverID)
	return q
}

// First executes the query and returns the first result
func (q *Query[T]) First() (*T, error) {
	var entity T
	err := q.db.First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &entity, nil
}

// FirstOrFail executes the query and returns an error if not found
func (q *Query[T]) FirstOrFail() (*T, error) {
	entity, err := q.First()
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, NotFoundError(q.modelName, "")
		}
		return nil, WrapError(err, q.modelName, fmt.Sprintf("failed to find %s", q.modelName))
	}
	return entity, nil
}

// All returns all matching records
func (q *Query[T]) All() ([]T, error) {
	var entities []T
	err := q.db.Find(&entities).Error
	return entities, err
}

// Count returns the count of matching records
func (q *Query[T]) Count() (int64, error) {
	var count int64
	var entity T
	err := q.db.Model(&entity).Count(&count).Error
	return count, err
}

// Exists checks if any matching record exists
func (q *Query[T]) Exists() (bool, error) {
	count, err := q.Count()
	return count > 0, err
}

// Delete deletes matching records
func (q *Query[T]) Delete() error {
	var entity T
	return q.db.Delete(&entity).Error
}

// getTypeName gets the type name using reflection
func getTypeName(entity interface{}) string {
	return fmt.Sprintf("%T", entity)
}

// DB returns the underlying gorm.DB for custom operations
func (q *Query[T]) DB() *gorm.DB {
	return q.db
}

// Find is a convenience function to find a record by ID or fail
func Find[T any](db *gorm.DB, ctx context.Context, id string) (*T, error) {
	return NewQuery[T](db, ctx).FindByID(id).FirstOrFail()
}

// FindWithPreload finds a record by ID with preloaded relations or fail
func FindWithPreload[T any](db *gorm.DB, ctx context.Context, id string, relations ...string) (*T, error) {
	return NewQuery[T](db, ctx).Preload(relations...).FindByID(id).FirstOrFail()
}

// FindByServer finds a record by ID and server ID or fail
func FindByServer[T any](db *gorm.DB, ctx context.Context, id, serverID string) (*T, error) {
	return NewQuery[T](db, ctx).FindByIDAndServer(id, serverID).FirstOrFail()
}
