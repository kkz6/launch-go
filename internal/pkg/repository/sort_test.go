package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupSortTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}

func TestParseSort(t *testing.T) {
	cfg := SortConfig{
		AllowedFields: map[string]string{
			"createdAt": "created_at",
			"name":      "name",
			"status":    "status",
		},
		DefaultField: "createdAt",
		DefaultDir:   "desc",
	}

	tests := []struct {
		name      string
		field     string
		direction string
		expected  SortParams
	}{
		{
			name:      "valid field and direction",
			field:     "name",
			direction: "asc",
			expected: SortParams{
				Field:     "name",
				Direction: "asc",
			},
		},
		{
			name:      "valid field with desc direction",
			field:     "status",
			direction: "desc",
			expected: SortParams{
				Field:     "status",
				Direction: "desc",
			},
		},
		{
			name:      "maps camelCase to snake_case",
			field:     "createdAt",
			direction: "desc",
			expected: SortParams{
				Field:     "created_at",
				Direction: "desc",
			},
		},
		{
			name:      "invalid field falls back to default",
			field:     "invalid",
			direction: "asc",
			expected: SortParams{
				Field:     "created_at",
				Direction: "asc",
			},
		},
		{
			name:      "empty field falls back to default",
			field:     "",
			direction: "asc",
			expected: SortParams{
				Field:     "created_at",
				Direction: "asc",
			},
		},
		{
			name:      "invalid direction falls back to default",
			field:     "name",
			direction: "invalid",
			expected: SortParams{
				Field:     "name",
				Direction: "desc",
			},
		},
		{
			name:      "empty direction falls back to default",
			field:     "name",
			direction: "",
			expected: SortParams{
				Field:     "name",
				Direction: "desc",
			},
		},
		{
			name:      "both empty fall back to defaults",
			field:     "",
			direction: "",
			expected: SortParams{
				Field:     "created_at",
				Direction: "desc",
			},
		},
		{
			name:      "handles whitespace",
			field:     "  name  ",
			direction: "  ASC  ",
			expected: SortParams{
				Field:     "name",
				Direction: "asc",
			},
		},
		{
			name:      "case insensitive direction",
			field:     "name",
			direction: "ASC",
			expected: SortParams{
				Field:     "name",
				Direction: "asc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSort(tt.field, tt.direction, cfg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortParams_Apply(t *testing.T) {
	db := setupSortTestDB(t)

	cfg := SortConfig{
		AllowedFields: map[string]string{
			"createdAt": "created_at",
			"name":      "name",
		},
		DefaultField: "createdAt",
		DefaultDir:   "desc",
	}

	tests := []struct {
		name          string
		params        SortParams
		expectedOrder string
	}{
		{
			name: "applies valid sort",
			params: SortParams{
				Field:     "name",
				Direction: "asc",
			},
			expectedOrder: "name ASC",
		},
		{
			name: "applies desc sort",
			params: SortParams{
				Field:     "created_at",
				Direction: "desc",
			},
			expectedOrder: "created_at DESC",
		},
		{
			name: "invalid field falls back to default",
			params: SortParams{
				Field:     "invalid_field",
				Direction: "asc",
			},
			expectedOrder: "created_at ASC",
		},
		{
			name: "invalid direction falls back to default",
			params: SortParams{
				Field:     "name",
				Direction: "invalid",
			},
			expectedOrder: "name DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.params.Apply(db, cfg)
			sql := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Find(&struct{}{})
			})
			assert.Contains(t, sql, tt.expectedOrder)
		})
	}
}

func TestSortParams_ApplyToQuery(t *testing.T) {
	db := setupSortTestDB(t)

	tests := []struct {
		name          string
		params        SortParams
		expectedOrder string
		expectNoOrder bool
	}{
		{
			name: "applies sort",
			params: SortParams{
				Field:     "name",
				Direction: "asc",
			},
			expectedOrder: "name ASC",
		},
		{
			name: "empty field skips order",
			params: SortParams{
				Field:     "",
				Direction: "asc",
			},
			expectNoOrder: true,
		},
		{
			name: "invalid direction defaults to desc",
			params: SortParams{
				Field:     "name",
				Direction: "invalid",
			},
			expectedOrder: "name DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.params.ApplyToQuery(db)
			sql := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Find(&struct{}{})
			})

			if tt.expectNoOrder {
				assert.NotContains(t, sql, "ORDER BY")
			} else {
				assert.Contains(t, sql, tt.expectedOrder)
			}
		})
	}
}

func TestSortScope(t *testing.T) {
	db := setupSortTestDB(t)

	cfg := SortConfig{
		AllowedFields: map[string]string{
			"name": "name",
		},
		DefaultField: "name",
		DefaultDir:   "asc",
	}

	params := SortParams{
		Field:     "name",
		Direction: "desc",
	}

	scope := SortScope(params, cfg)
	query := scope(db)

	sql := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(&struct{}{})
	})

	assert.Contains(t, sql, "name DESC")
}

func TestDefaultSortConfig(t *testing.T) {
	cfg := DefaultSortConfig()

	assert.Equal(t, "createdAt", cfg.DefaultField)
	assert.Equal(t, "desc", cfg.DefaultDir)
	assert.Equal(t, "created_at", cfg.AllowedFields["createdAt"])
	assert.Equal(t, "updated_at", cfg.AllowedFields["updatedAt"])
	assert.Equal(t, "name", cfg.AllowedFields["name"])
}

func TestNewSortConfig(t *testing.T) {
	fields := map[string]string{
		"title":  "title",
		"rating": "rating",
	}

	cfg := NewSortConfig(fields, "title", "asc")

	assert.Equal(t, "title", cfg.DefaultField)
	assert.Equal(t, "asc", cfg.DefaultDir)
	assert.Equal(t, fields, cfg.AllowedFields)

	// Test default direction
	cfg2 := NewSortConfig(fields, "title", "")
	assert.Equal(t, "desc", cfg2.DefaultDir)
}
