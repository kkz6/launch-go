package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupFilterTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}

func toSQL(db *gorm.DB) string {
	return db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(&struct{}{})
	})
}

func TestApplyFilters(t *testing.T) {
	db := setupFilterTestDB(t)

	t.Run("applies multiple filters", func(t *testing.T) {
		query := ApplyFilters(db,
			WithStatusFilter("active"),
			WithOptionalLimit(10),
		)
		sql := toSQL(query)
		assert.Contains(t, sql, "status")
		assert.Contains(t, sql, "active")
		assert.Contains(t, sql, "LIMIT 10")
	})

	t.Run("ignores nil filters", func(t *testing.T) {
		query := ApplyFilters(db,
			nil,
			WithStatusFilter("active"),
			nil,
		)
		sql := toSQL(query)
		assert.Contains(t, sql, "status")
	})
}

func TestWithDateRange(t *testing.T) {
	db := setupFilterTestDB(t)
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	tests := []struct {
		name       string
		column     string
		from       *time.Time
		to         *time.Time
		expectFrom bool
		expectTo   bool
	}{
		{
			name:       "both bounds",
			column:     "created_at",
			from:       &yesterday,
			to:         &now,
			expectFrom: true,
			expectTo:   true,
		},
		{
			name:       "only from",
			column:     "created_at",
			from:       &yesterday,
			to:         nil,
			expectFrom: true,
			expectTo:   false,
		},
		{
			name:       "only to",
			column:     "created_at",
			from:       nil,
			to:         &now,
			expectFrom: false,
			expectTo:   true,
		},
		{
			name:       "neither bound",
			column:     "created_at",
			from:       nil,
			to:         nil,
			expectFrom: false,
			expectTo:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithDateRange(tt.column, tt.from, tt.to))
			sql := toSQL(query)

			if tt.expectFrom {
				assert.Contains(t, sql, ">=")
			}
			if tt.expectTo {
				assert.Contains(t, sql, "<=")
			}
			if !tt.expectFrom && !tt.expectTo {
				assert.NotContains(t, sql, "created_at")
			}
		})
	}
}

func TestWithOptionalLimit(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name        string
		limit       int
		expectLimit bool
	}{
		{
			name:        "positive limit is applied",
			limit:       10,
			expectLimit: true,
		},
		{
			name:        "zero limit is not applied",
			limit:       0,
			expectLimit: false,
		},
		{
			name:        "negative limit is not applied",
			limit:       -5,
			expectLimit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithOptionalLimit(tt.limit))
			sql := toSQL(query)

			if tt.expectLimit {
				assert.Contains(t, sql, "LIMIT")
			} else {
				assert.NotContains(t, sql, "LIMIT")
			}
		})
	}
}

func TestWithStatusFilter(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name         string
		status       string
		expectFilter bool
	}{
		{
			name:         "non-empty status is applied",
			status:       "active",
			expectFilter: true,
		},
		{
			name:         "empty status is not applied",
			status:       "",
			expectFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithStatusFilter(tt.status))
			sql := toSQL(query)

			if tt.expectFilter {
				assert.Contains(t, sql, "status")
				assert.Contains(t, sql, tt.status)
			} else {
				assert.NotContains(t, sql, "status")
			}
		})
	}
}

func TestWithStatuses(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name         string
		statuses     []string
		expectFilter bool
	}{
		{
			name:         "multiple statuses",
			statuses:     []string{"active", "pending"},
			expectFilter: true,
		},
		{
			name:         "single status",
			statuses:     []string{"active"},
			expectFilter: true,
		},
		{
			name:         "empty slice is not applied",
			statuses:     []string{},
			expectFilter: false,
		},
		{
			name:         "nil slice is not applied",
			statuses:     nil,
			expectFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithStatuses(tt.statuses))
			sql := toSQL(query)

			if tt.expectFilter {
				assert.Contains(t, sql, "status")
				assert.Contains(t, sql, "IN")
			} else {
				assert.NotContains(t, sql, "status")
			}
		})
	}
}

func TestWithPagination(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name         string
		limit        int
		offset       int
		expectLimit  bool
		expectOffset bool
	}{
		{
			name:         "both limit and offset",
			limit:        10,
			offset:       20,
			expectLimit:  true,
			expectOffset: true,
		},
		{
			name:         "only limit",
			limit:        10,
			offset:       0,
			expectLimit:  true,
			expectOffset: false,
		},
		{
			name:         "neither",
			limit:        0,
			offset:       0,
			expectLimit:  false,
			expectOffset: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithPagination(tt.limit, tt.offset))
			sql := toSQL(query)

			if tt.expectLimit {
				assert.Contains(t, sql, "LIMIT")
			} else {
				assert.NotContains(t, sql, "LIMIT")
			}
			if tt.expectOffset {
				assert.Contains(t, sql, "OFFSET")
			}
		})
	}
}

func TestWithSearch(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name         string
		column       string
		search       string
		expectFilter bool
	}{
		{
			name:         "non-empty search",
			column:       "name",
			search:       "test",
			expectFilter: true,
		},
		{
			name:         "empty search is not applied",
			column:       "name",
			search:       "",
			expectFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithSearch(tt.column, tt.search))
			sql := toSQL(query)

			if tt.expectFilter {
				assert.Contains(t, sql, "LIKE")
				assert.Contains(t, sql, tt.column)
			} else {
				assert.NotContains(t, sql, "LIKE")
			}
		})
	}
}

func TestWithColumnValue(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name         string
		column       string
		value        interface{}
		expectFilter bool
	}{
		{
			name:         "string value",
			column:       "status",
			value:        "active",
			expectFilter: true,
		},
		{
			name:         "int value",
			column:       "count",
			value:        42,
			expectFilter: true,
		},
		{
			name:         "empty string is not applied",
			column:       "status",
			value:        "",
			expectFilter: false,
		},
		{
			name:         "nil is not applied",
			column:       "status",
			value:        nil,
			expectFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithColumnValue(tt.column, tt.value))
			sql := toSQL(query)

			if tt.expectFilter {
				assert.Contains(t, sql, tt.column)
			}
		})
	}
}

func TestWithColumnNull(t *testing.T) {
	db := setupFilterTestDB(t)

	query := ApplyFilters(db, WithColumnNull("deleted_at"))
	sql := toSQL(query)
	assert.Contains(t, sql, "deleted_at IS NULL")
}

func TestWithColumnNotNull(t *testing.T) {
	db := setupFilterTestDB(t)

	query := ApplyFilters(db, WithColumnNotNull("deleted_at"))
	sql := toSQL(query)
	assert.Contains(t, sql, "deleted_at IS NOT NULL")
}

func TestWithColumnBetween(t *testing.T) {
	db := setupFilterTestDB(t)

	query := ApplyFilters(db, WithColumnBetween("price", 10, 100))
	sql := toSQL(query)
	assert.Contains(t, sql, "price BETWEEN")
}

func TestWithOrder(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name      string
		column    string
		direction string
		expected  string
	}{
		{
			name:      "order asc",
			column:    "name",
			direction: "ASC",
			expected:  "name ASC",
		},
		{
			name:      "order desc",
			column:    "created_at",
			direction: "DESC",
			expected:  "created_at DESC",
		},
		{
			name:      "empty column is not applied",
			column:    "",
			direction: "ASC",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithOrder(tt.column, tt.direction))
			sql := toSQL(query)

			if tt.expected != "" {
				assert.Contains(t, sql, tt.expected)
			} else {
				assert.NotContains(t, sql, "ORDER BY")
			}
		})
	}
}

func TestWithPreload(t *testing.T) {
	db := setupFilterTestDB(t)

	// Preload doesn't appear in ToSQL, but we can verify it doesn't error
	query := ApplyFilters(db, WithPreload("Servers"))
	assert.NotNil(t, query)
}

func TestWithMultiplePreloads(t *testing.T) {
	db := setupFilterTestDB(t)

	query := ApplyFilters(db, WithMultiplePreloads("Servers", "Sites", "Users"))
	assert.NotNil(t, query)
}

func TestOnlyActive(t *testing.T) {
	db := setupFilterTestDB(t)

	query := ApplyFilters(db, OnlyActive())
	sql := toSQL(query)
	assert.Contains(t, sql, "archived_at IS NULL")
}

func TestOnlyArchived(t *testing.T) {
	db := setupFilterTestDB(t)

	query := ApplyFilters(db, OnlyArchived())
	sql := toSQL(query)
	assert.Contains(t, sql, "archived_at IS NOT NULL")
}

func TestConditionalFilter(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name         string
		condition    bool
		expectFilter bool
	}{
		{
			name:         "condition true applies filter",
			condition:    true,
			expectFilter: true,
		},
		{
			name:         "condition false skips filter",
			condition:    false,
			expectFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, ConditionalFilter(tt.condition, WithStatusFilter("active")))
			sql := toSQL(query)

			if tt.expectFilter {
				assert.Contains(t, sql, "status")
			} else {
				assert.NotContains(t, sql, "status")
			}
		})
	}
}

func TestWithIDsFilter(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name         string
		ids          []string
		expectFilter bool
	}{
		{
			name:         "with ids",
			ids:          []string{"id1", "id2", "id3"},
			expectFilter: true,
		},
		{
			name:         "empty ids",
			ids:          []string{},
			expectFilter: false,
		},
		{
			name:         "nil ids",
			ids:          nil,
			expectFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithIDsFilter(tt.ids))
			sql := toSQL(query)

			if tt.expectFilter {
				assert.Contains(t, sql, "id IN")
			} else {
				assert.NotContains(t, sql, "id IN")
			}
		})
	}
}

func TestWithTypes(t *testing.T) {
	db := setupFilterTestDB(t)

	tests := []struct {
		name         string
		types        []string
		expectFilter bool
	}{
		{
			name:         "with types",
			types:        []string{"laravel", "wordpress"},
			expectFilter: true,
		},
		{
			name:         "empty types",
			types:        []string{},
			expectFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := ApplyFilters(db, WithTypes(tt.types))
			sql := toSQL(query)

			if tt.expectFilter {
				assert.Contains(t, sql, "type IN")
			} else {
				assert.NotContains(t, sql, "type IN")
			}
		})
	}
}

func TestFilterChaining(t *testing.T) {
	db := setupFilterTestDB(t)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	// Test complex filter chain
	query := ApplyFilters(db,
		WithTeamIDFilter("team123"),
		WithStatuses([]string{"active", "pending"}),
		WithDateRange("created_at", &yesterday, &now),
		WithOptionalLimit(25),
		WithOptionalOffset(50),
		WithOrderDesc("created_at"),
		OnlyActive(),
	)

	sql := toSQL(query)

	assert.Contains(t, sql, "team_id")
	assert.Contains(t, sql, "status")
	assert.Contains(t, sql, "IN")
	assert.Contains(t, sql, ">=")
	assert.Contains(t, sql, "<=")
	assert.Contains(t, sql, "LIMIT 25")
	assert.Contains(t, sql, "OFFSET 50")
	assert.Contains(t, sql, "ORDER BY")
	assert.Contains(t, sql, "archived_at IS NULL")
}
