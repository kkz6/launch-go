package models

import (
	"time"

	"gorm.io/gorm"
)

// ArchivableEntity defines the interface for models that support soft archival.
// Implement this interface for type-safe handling of archivable models.
//
// Example usage:
//
//	func archiveIfNeeded(model ArchivableEntity) {
//	    if !model.IsArchived() {
//	        model.Archive()
//	    }
//	}
type ArchivableEntity interface {
	// IsArchived returns true if the model has been archived
	IsArchived() bool

	// Archive marks the model as archived with the current timestamp
	Archive()

	// Unarchive removes the archived status from the model
	Unarchive()

	// GetArchivedAt returns the archived timestamp, nil if not archived
	GetArchivedAt() *time.Time
}

// Compile-time check that ArchivableModel implements ArchivableEntity
var _ ArchivableEntity = (*ArchivableModel)(nil)

// ArchivableModel provides soft-archive functionality for models that should not be deleted
// but should be hidden from normal queries. Unlike GORM's soft delete (DeletedAt), archived
// records are not automatically filtered - you must explicitly use scopes.
//
// This is the preferred pattern for soft deletion in this codebase. Use this instead of
// GORM's DeletedAt or status-based deletion.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.ArchivableModel
//	}
//
//	// Query only active records
//	db.Scopes(NotArchived).Find(&records)
//
//	// Query only archived records
//	db.Scopes(OnlyArchived).Find(&records)
type ArchivableModel struct {
	ArchivedAt *time.Time `gorm:"column:archived_at;type:timestamp null;index" json:"archived_at,omitempty"`
}

// IsArchived returns true if the model has been archived
func (m *ArchivableModel) IsArchived() bool {
	return m.ArchivedAt != nil
}

// Archive marks the model as archived with the current timestamp
func (m *ArchivableModel) Archive() {
	now := time.Now()
	m.ArchivedAt = &now
}

// Unarchive removes the archived status from the model
func (m *ArchivableModel) Unarchive() {
	m.ArchivedAt = nil
}

// GetArchivedAt returns the archived timestamp
func (m *ArchivableModel) GetArchivedAt() *time.Time {
	return m.ArchivedAt
}

// GORM Scopes for Archivable Models
//
// These scopes should be used when querying archivable models to control
// whether archived records are included in results.

// NotArchived returns a scope that filters out archived records.
// Use this for most queries where you want only active records.
//
// Example:
//
//	db.Scopes(NotArchived).Find(&servers)
func NotArchived(db *gorm.DB) *gorm.DB {
	return db.Where("archived_at IS NULL")
}

// OnlyArchived returns a scope that returns only archived records.
// Use this for "trash" or "archive" views.
//
// Example:
//
//	db.Scopes(OnlyArchived).Find(&archivedServers)
func OnlyArchived(db *gorm.DB) *gorm.DB {
	return db.Where("archived_at IS NOT NULL")
}

// WithArchiveStatus returns a scope that filters by archive status.
// Pass true to get only archived records, false for only active records.
//
// Example:
//
//	includeArchived := request.Query("include_archived") == "true"
//	db.Scopes(WithArchiveStatus(includeArchived)).Find(&records)
func WithArchiveStatus(archived bool) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if archived {
			return OnlyArchived(db)
		}
		return NotArchived(db)
	}
}

// IncludingArchived is a no-op scope that documents the intent to include
// all records regardless of archive status. Use this for clarity when you
// explicitly want to include archived records.
//
// Example:
//
//	db.Scopes(IncludingArchived).Find(&allRecords)
func IncludingArchived(db *gorm.DB) *gorm.DB {
	return db
}
