package repository

import (
	"strings"

	"gorm.io/gorm"
)

// SortConfig configures which fields are allowed for sorting
// and provides defaults when no sort is specified.
type SortConfig struct {
	// AllowedFields maps user-facing field names to actual DB column names.
	// Example: {"createdAt": "created_at", "name": "name"}
	AllowedFields map[string]string

	// DefaultField is the field to sort by when none is specified.
	DefaultField string

	// DefaultDir is the direction to sort when none is specified ("asc" or "desc").
	DefaultDir string
}

// SortParams holds the parsed sort field and direction.
type SortParams struct {
	Field     string
	Direction string
}

// ParseSort parses sort parameters from raw field and direction strings.
// It validates and normalizes the values against the provided config.
func ParseSort(field, direction string, cfg SortConfig) SortParams {
	// Normalize direction
	dir := strings.ToLower(strings.TrimSpace(direction))
	if dir != "asc" && dir != "desc" {
		dir = cfg.DefaultDir
		if dir == "" {
			dir = "desc"
		}
	}

	// Validate and map field
	field = strings.TrimSpace(field)
	dbColumn, ok := cfg.AllowedFields[field]
	if !ok || dbColumn == "" {
		// Use default field
		dbColumn = cfg.AllowedFields[cfg.DefaultField]
		if dbColumn == "" {
			dbColumn = cfg.DefaultField
		}
	}

	return SortParams{
		Field:     dbColumn,
		Direction: dir,
	}
}

// Apply applies the sort parameters to a GORM query.
// Returns the modified query with ORDER BY clause applied.
func (s SortParams) Apply(db *gorm.DB, cfg SortConfig) *gorm.DB {
	// Validate field is in allowed list
	isAllowed := false
	for _, col := range cfg.AllowedFields {
		if col == s.Field {
			isAllowed = true
			break
		}
	}

	field := s.Field
	dir := s.Direction

	// Fall back to defaults if field is not allowed
	if !isAllowed {
		field = cfg.AllowedFields[cfg.DefaultField]
		if field == "" {
			field = cfg.DefaultField
		}
	}

	// Normalize direction
	if dir != "asc" && dir != "desc" {
		dir = cfg.DefaultDir
		if dir == "" {
			dir = "desc"
		}
	}

	return db.Order(field + " " + strings.ToUpper(dir))
}

// ApplyToQuery applies sort params without requiring config validation.
// Use this when you've already validated the params via ParseSort.
func (s SortParams) ApplyToQuery(db *gorm.DB) *gorm.DB {
	if s.Field == "" {
		return db
	}

	dir := s.Direction
	if dir != "asc" && dir != "desc" {
		dir = "desc"
	}

	return db.Order(s.Field + " " + strings.ToUpper(dir))
}

// SortScope returns a Scope function that applies sorting.
// This integrates with the existing Scope pattern in scopes.go.
func SortScope(params SortParams, cfg SortConfig) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return params.Apply(db, cfg)
	}
}

// DefaultSortConfig returns a common sort config for created_at ordering.
func DefaultSortConfig() SortConfig {
	return SortConfig{
		AllowedFields: map[string]string{
			"createdAt": "created_at",
			"updatedAt": "updated_at",
			"name":      "name",
		},
		DefaultField: "createdAt",
		DefaultDir:   "desc",
	}
}

// NewSortConfig creates a SortConfig with the given allowed fields and defaults.
func NewSortConfig(allowedFields map[string]string, defaultField, defaultDir string) SortConfig {
	if defaultDir == "" {
		defaultDir = "desc"
	}

	return SortConfig{
		AllowedFields: allowedFields,
		DefaultField:  defaultField,
		DefaultDir:    defaultDir,
	}
}
