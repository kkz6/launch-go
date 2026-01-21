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
func NewQuery[T any](ctx context.Context, db *gorm.DB) *Query[T] {
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

// Scopes applies one or more scope functions to the query
func (q *Query[T]) Scopes(scopes ...Scope) *Query[T] {
	for _, scope := range scopes {
		q.db = scope(q.db)
	}
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

// OrderBy adds an ORDER BY clause
func (q *Query[T]) OrderBy(column, direction string) *Query[T] {
	q.db = q.db.Order(column + " " + direction)
	return q
}

// OrderByDesc adds an ORDER BY DESC clause
func (q *Query[T]) OrderByDesc(column string) *Query[T] {
	return q.OrderBy(column, "DESC")
}

// OrderByAsc adds an ORDER BY ASC clause
func (q *Query[T]) OrderByAsc(column string) *Query[T] {
	return q.OrderBy(column, "ASC")
}

// Limit sets the maximum number of records to return
func (q *Query[T]) Limit(n int) *Query[T] {
	q.db = q.db.Limit(n)
	return q
}

// Offset sets the number of records to skip
func (q *Query[T]) Offset(n int) *Query[T] {
	q.db = q.db.Offset(n)
	return q
}

// Select specifies which columns to retrieve
func (q *Query[T]) Select(columns ...string) *Query[T] {
	q.db = q.db.Select(columns)
	return q
}

// Omit specifies columns to omit from the result
func (q *Query[T]) Omit(columns ...string) *Query[T] {
	q.db = q.db.Omit(columns...)
	return q
}

// Join adds a JOIN clause
func (q *Query[T]) Join(join string, args ...interface{}) *Query[T] {
	q.db = q.db.Joins(join, args...)
	return q
}

// Group adds a GROUP BY clause
func (q *Query[T]) Group(column string) *Query[T] {
	q.db = q.db.Group(column)
	return q
}

// Having adds a HAVING clause
func (q *Query[T]) Having(query interface{}, args ...interface{}) *Query[T] {
	q.db = q.db.Having(query, args...)
	return q
}

// Distinct marks the query as DISTINCT
func (q *Query[T]) Distinct(columns ...string) *Query[T] {
	if len(columns) > 0 {
		q.db = q.db.Distinct(columns)
	} else {
		q.db = q.db.Distinct()
	}
	return q
}

// Or adds an OR clause to the query
func (q *Query[T]) Or(query interface{}, args ...interface{}) *Query[T] {
	q.db = q.db.Or(query, args...)
	return q
}

// Not adds a NOT clause to the query
func (q *Query[T]) Not(query interface{}, args ...interface{}) *Query[T] {
	q.db = q.db.Not(query, args...)
	return q
}

// WhereIn adds a WHERE IN clause
func (q *Query[T]) WhereIn(column string, values interface{}) *Query[T] {
	q.db = q.db.Where(column+" IN ?", values)
	return q
}

// WhereNotIn adds a WHERE NOT IN clause
func (q *Query[T]) WhereNotIn(column string, values interface{}) *Query[T] {
	q.db = q.db.Where(column+" NOT IN ?", values)
	return q
}

// WhereNull adds a WHERE IS NULL clause
func (q *Query[T]) WhereNull(column string) *Query[T] {
	q.db = q.db.Where(column + " IS NULL")
	return q
}

// WhereNotNull adds a WHERE IS NOT NULL clause
func (q *Query[T]) WhereNotNull(column string) *Query[T] {
	q.db = q.db.Where(column + " IS NOT NULL")
	return q
}

// WhereBetween adds a WHERE BETWEEN clause
func (q *Query[T]) WhereBetween(column string, minVal, maxVal interface{}) *Query[T] {
	q.db = q.db.Where(column+" BETWEEN ? AND ?", minVal, maxVal)
	return q
}

// WhereLike adds a WHERE LIKE clause
func (q *Query[T]) WhereLike(column string, pattern string) *Query[T] {
	q.db = q.db.Where(column+" LIKE ?", pattern)
	return q
}

// Paginate returns paginated results with total count
func (q *Query[T]) Paginate(page, perPage int) ([]T, int64, error) {
	// Get total count first (before limit/offset)
	total, err := q.Count()
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * perPage
	q.db = q.db.Offset(offset).Limit(perPage)

	// Get results
	results, err := q.All()
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// Pluck retrieves a single column from all matching records
func (q *Query[T]) Pluck(column string, dest interface{}) error {
	var model T
	return q.db.Model(&model).Pluck(column, dest).Error
}

// Update updates matching records with the given values
func (q *Query[T]) Update(values map[string]interface{}) error {
	var model T
	return q.db.Model(&model).Updates(values).Error
}

// Debug enables debug mode for this query
func (q *Query[T]) Debug() *Query[T] {
	q.db = q.db.Debug()
	return q
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
func Find[T any](ctx context.Context, db *gorm.DB, id string) (*T, error) {
	return NewQuery[T](ctx, db).FindByID(id).FirstOrFail()
}

// FindWithPreload finds a record by ID with preloaded relations or fail
func FindWithPreload[T any](ctx context.Context, db *gorm.DB, id string, relations ...string) (*T, error) {
	return NewQuery[T](ctx, db).Preload(relations...).FindByID(id).FirstOrFail()
}

// FindByServer finds a record by ID and server ID or fail
func FindByServer[T any](ctx context.Context, db *gorm.DB, id, serverID string) (*T, error) {
	return NewQuery[T](ctx, db).FindByIDAndServer(id, serverID).FirstOrFail()
}
