package repository

import (
	"context"
	"errors"
	"reflect"

	"gorm.io/gorm"
)

// Base provides common database operations using generics.
// Embed this in your repository to get common CRUD operations.
//
// Usage:
//
//	type DatabaseRepository struct {
//	    repository.Base[models.Database]
//	}
type Base[T any] struct {
	DB *gorm.DB
}

// NewBase creates a new Base repository
func NewBase[T any](db *gorm.DB) Base[T] {
	return Base[T]{DB: db}
}

// Create creates a new record
func (r *Base[T]) Create(ctx context.Context, entity *T) error {
	return r.DB.WithContext(ctx).Create(entity).Error
}

// FindByID finds a record by ID
func (r *Base[T]) FindByID(ctx context.Context, id string) (*T, error) {
	var entity T
	err := r.DB.WithContext(ctx).First(&entity, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &entity, nil
}

// FindByIDAndServer finds a record by ID and server ID
func (r *Base[T]) FindByIDAndServer(ctx context.Context, id, serverID string) (*T, error) {
	var entity T
	err := r.DB.WithContext(ctx).First(&entity, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &entity, nil
}

// FindByServer finds all records for a server
func (r *Base[T]) FindByServer(ctx context.Context, serverID string) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}

// FindByTeam finds all records for a team
func (r *Base[T]) FindByTeam(ctx context.Context, teamID string) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}

// FindByUser finds all records for a user
func (r *Base[T]) FindByUser(ctx context.Context, userID string) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&entities).Error
	return entities, err
}

// Update updates a record
func (r *Base[T]) Update(ctx context.Context, entity *T) error {
	return r.DB.WithContext(ctx).Save(entity).Error
}

// UpdateFields updates specific fields of a record
func (r *Base[T]) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	var entity T
	return r.DB.WithContext(ctx).
		Model(&entity).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete deletes a record by ID
func (r *Base[T]) Delete(ctx context.Context, id string) error {
	var entity T
	return r.DB.WithContext(ctx).Delete(&entity, "id = ?", id).Error
}

// Exists checks if a record exists by ID
func (r *Base[T]) Exists(ctx context.Context, id string) (bool, error) {
	var count int64
	var entity T
	err := r.DB.WithContext(ctx).
		Model(&entity).
		Where("id = ?", id).
		Count(&count).Error
	return count > 0, err
}

// ExistsByNameAndServer checks if a record exists with the given name on the server
func (r *Base[T]) ExistsByNameAndServer(ctx context.Context, name, serverID string) (bool, error) {
	var count int64
	var entity T
	err := r.DB.WithContext(ctx).
		Model(&entity).
		Where("name = ? AND server_id = ?", name, serverID).
		Count(&count).Error
	return count > 0, err
}

// FindByNameAndServer finds a record by name and server ID
func (r *Base[T]) FindByNameAndServer(ctx context.Context, name, serverID string) (*T, error) {
	var entity T
	err := r.DB.WithContext(ctx).
		First(&entity, "name = ? AND server_id = ?", name, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &entity, nil
}

// Count returns the total count of records
func (r *Base[T]) Count(ctx context.Context) (int64, error) {
	var count int64
	var entity T
	err := r.DB.WithContext(ctx).Model(&entity).Count(&count).Error
	return count, err
}

// CountByServer returns the count of records for a server
func (r *Base[T]) CountByServer(ctx context.Context, serverID string) (int64, error) {
	var count int64
	var entity T
	err := r.DB.WithContext(ctx).
		Model(&entity).
		Where("server_id = ?", serverID).
		Count(&count).Error
	return count, err
}

// Transaction executes a function within a database transaction
func (r *Base[T]) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.DB.WithContext(ctx).Transaction(fn)
}

// Query returns the underlying DB for custom queries
func (r *Base[T]) Query(ctx context.Context) *gorm.DB {
	return r.DB.WithContext(ctx)
}

// WithPreload returns a query with preloaded relations
func (r *Base[T]) WithPreload(ctx context.Context, relations ...string) *gorm.DB {
	query := r.DB.WithContext(ctx)
	for _, rel := range relations {
		query = query.Preload(rel)
	}
	return query
}

// getModelName returns the type name of T for error messages
func (r *Base[T]) getModelName() string {
	var entity T
	t := reflect.TypeOf(entity)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// FindByIDOrFail finds a record by ID or returns a typed error
func (r *Base[T]) FindByIDOrFail(ctx context.Context, id string) (*T, error) {
	entity, err := r.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, NotFoundError(r.getModelName(), id)
		}
		return nil, WrapError(err, r.getModelName(), "failed to find "+r.getModelName())
	}
	return entity, nil
}

// FindByIDAndServerOrFail finds a record by ID and server ID or returns a typed error
func (r *Base[T]) FindByIDAndServerOrFail(ctx context.Context, id, serverID string) (*T, error) {
	entity, err := r.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, NotFoundError(r.getModelName(), id)
		}
		return nil, WrapError(err, r.getModelName(), "failed to find "+r.getModelName())
	}
	return entity, nil
}

// FirstOrFail executes a query and returns an error if not found
func (r *Base[T]) FirstOrFail(ctx context.Context, query *gorm.DB) (*T, error) {
	var entity T
	err := query.WithContext(ctx).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NotFoundError(r.getModelName(), "")
		}
		return nil, WrapError(err, r.getModelName(), "failed to find "+r.getModelName())
	}
	return &entity, nil
}

// MustFind is an alias for FindByIDOrFail (more idiomatic Go naming)
func (r *Base[T]) MustFind(ctx context.Context, id string) (*T, error) {
	return r.FindByIDOrFail(ctx, id)
}
