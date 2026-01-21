package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Scope is a function that modifies a GORM query.
// Scopes can be composed together to build complex queries.
type Scope func(*gorm.DB) *gorm.DB

// WithID returns a scope that filters by ID
func WithID(id string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id)
	}
}

// WithServerID returns a scope that filters by server_id
func WithServerID(serverID string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("server_id = ?", serverID)
	}
}

// WithTeamID returns a scope that filters by team_id
func WithTeamID(teamID string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("team_id = ?", teamID)
	}
}

// WithUserID returns a scope that filters by user_id
func WithUserID(userID string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("user_id = ?", userID)
	}
}

// WithSiteID returns a scope that filters by site_id
func WithSiteID(siteID string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("site_id = ?", siteID)
	}
}

// WithName returns a scope that filters by name
func WithName(name string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("name = ?", name)
	}
}

// WithAddress returns a scope that filters by address
func WithAddress(address string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("address = ?", address)
	}
}

// WithStatus returns a scope that filters by status
func WithStatus(status string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", status)
	}
}

// WithType returns a scope that filters by type
func WithType(t string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("type = ?", t)
	}
}

// OrderByCreatedDesc returns a scope that orders by created_at descending
func OrderByCreatedDesc() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}
}

// OrderByCreatedAsc returns a scope that orders by created_at ascending
func OrderByCreatedAsc() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}
}

// OrderBy returns a scope that orders by a specific column
func OrderBy(column, direction string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order(column + " " + direction)
	}
}

// Limit returns a scope that limits results
func Limit(n int) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Limit(n)
	}
}

// Offset returns a scope that adds an offset
func Offset(n int) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(n)
	}
}

// Preload returns a scope that preloads a relation
func Preload(relation string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Preload(relation)
	}
}

// PreloadMany returns a scope that preloads multiple relations
func PreloadMany(relations ...string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		for _, rel := range relations {
			db = db.Preload(rel)
		}
		return db
	}
}

// PreloadOrdered returns a scope that preloads a relation with custom ordering
func PreloadOrdered(relation string, order string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Preload(relation, func(db *gorm.DB) *gorm.DB {
			return db.Order(order)
		})
	}
}

// PreloadWithScope returns a scope that preloads a relation with a custom scope
func PreloadWithScope(relation string, scope Scope) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Preload(relation, func(db *gorm.DB) *gorm.DB {
			return scope(db)
		})
	}
}

// WhereNotNull returns a scope that filters for non-null column values
func WhereNotNull(column string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(column + " IS NOT NULL")
	}
}

// WhereNull returns a scope that filters for null column values
func WhereNull(column string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(column + " IS NULL")
	}
}

// WhereIn returns a scope that filters by values in a list
func WhereIn(column string, values []string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(column+" IN ?", values)
	}
}

// Active returns a scope for active records (not deleted, status = active)
func Active() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ? OR status IS NULL", "active")
	}
}

// Archivable Model Scopes
//
// These scopes work with models that embed ArchivableModel or have an archived_at field.
// See internal/pkg/models/archivable.go for the model mixin.

// WithActive returns a scope for non-archived records (archived_at IS NULL).
// Use this for most queries where you want only active records.
//
// Example:
//
//	servers, _ := FindAll[Server](ctx, db, WithActive(), WithTeamID(teamID))
func WithActive() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("archived_at IS NULL")
	}
}

// WithArchived returns a scope for archived records (archived_at IS NOT NULL).
// Use this for "trash" or "archive" views.
//
// Example:
//
//	archived, _ := FindAll[Server](ctx, db, WithArchived(), WithTeamID(teamID))
func WithArchived() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("archived_at IS NOT NULL")
	}
}

// WithArchiveFilter returns a scope that filters by archive status.
// Pass true to get only archived records, false for only active records.
//
// Example:
//
//	showArchived := c.QueryBool("archived")
//	results, _ := FindAll[Server](ctx, db, WithArchiveFilter(showArchived))
func WithArchiveFilter(archived bool) Scope {
	if archived {
		return WithArchived()
	}
	return WithActive()
}

// IncludingArchived returns an empty scope that explicitly documents the intent
// to include all records regardless of archive status.
//
// Example:
//
//	allRecords, _ := FindAll[Server](ctx, db, IncludingArchived())
func IncludingArchived() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db
	}
}

// OrderByLatest returns a scope that orders by created_at descending
func OrderByLatest() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}
}

// OrderByOldest returns a scope that orders by created_at ascending
func OrderByOldest() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}
}

// applyScopes applies all scopes to a query
func applyScopes(db *gorm.DB, scopes ...Scope) *gorm.DB {
	for _, scope := range scopes {
		db = scope(db)
	}
	return db
}

// FindOne finds a single record matching the given scopes.
// Returns ErrNotFound if no record matches.
func FindOne[T any](ctx context.Context, db *gorm.DB, scopes ...Scope) (*T, error) {
	var entity T
	query := applyScopes(db.WithContext(ctx), scopes...)
	err := query.First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &entity, nil
}

// FindOneOrFail finds a single record or returns a typed NotFoundError.
func FindOneOrFail[T any](ctx context.Context, db *gorm.DB, scopes ...Scope) (*T, error) {
	entity, err := FindOne[T](ctx, db, scopes...)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			var t T
			return nil, NotFoundError(getTypeName(t), "")
		}
		return nil, err
	}
	return entity, nil
}

// FindAll finds all records matching the given scopes.
func FindAll[T any](ctx context.Context, db *gorm.DB, scopes ...Scope) ([]T, error) {
	var entities []T
	query := applyScopes(db.WithContext(ctx), scopes...)
	err := query.Find(&entities).Error
	return entities, err
}

// Count returns the count of records matching the given scopes.
func Count[T any](ctx context.Context, db *gorm.DB, scopes ...Scope) (int64, error) {
	var entity T
	var count int64
	query := applyScopes(db.WithContext(ctx), scopes...)
	err := query.Model(&entity).Count(&count).Error
	return count, err
}

// Exists checks if any record matches the given scopes.
func Exists[T any](ctx context.Context, db *gorm.DB, scopes ...Scope) (bool, error) {
	count, err := Count[T](ctx, db, scopes...)
	return count > 0, err
}

// DeleteAll deletes all records matching the given scopes.
func DeleteAll[T any](ctx context.Context, db *gorm.DB, scopes ...Scope) error {
	var entity T
	query := applyScopes(db.WithContext(ctx), scopes...)
	return query.Delete(&entity).Error
}

// UpdateAll updates all records matching the given scopes with the provided fields.
func UpdateAll[T any](ctx context.Context, db *gorm.DB, fields map[string]interface{}, scopes ...Scope) error {
	var entity T
	query := applyScopes(db.WithContext(ctx), scopes...)
	return query.Model(&entity).Updates(fields).Error
}
