package repository

import (
	"time"

	"gorm.io/gorm"
)

// FilterOption is a function that modifies a GORM query.
// FilterOptions are designed to be composable and can be combined
// using ApplyFilters.
type FilterOption func(*gorm.DB) *gorm.DB

// ApplyFilters applies all filter options to the query.
// Nil options are safely ignored.
func ApplyFilters(db *gorm.DB, opts ...FilterOption) *gorm.DB {
	for _, opt := range opts {
		if opt != nil {
			db = opt(db)
		}
	}
	return db
}

// WithDateRange adds a date range filter on the specified column.
// Both from and to are optional - if nil, that bound is not applied.
func WithDateRange(column string, from, to *time.Time) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if from != nil {
			db = db.Where(column+" >= ?", *from)
		}
		if to != nil {
			db = db.Where(column+" <= ?", *to)
		}
		return db
	}
}

// WithDateRangeCreated adds a date range filter on the created_at column.
func WithDateRangeCreated(from, to *time.Time) FilterOption {
	return WithDateRange("created_at", from, to)
}

// WithOptionalLimit applies a limit only if the value is greater than 0.
func WithOptionalLimit(limit int) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if limit > 0 {
			return db.Limit(limit)
		}
		return db
	}
}

// WithOptionalOffset applies an offset only if the value is greater than 0.
func WithOptionalOffset(offset int) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if offset > 0 {
			return db.Offset(offset)
		}
		return db
	}
}

// WithPagination applies limit and offset for pagination.
func WithPagination(limit, offset int) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if limit > 0 {
			db = db.Limit(limit)
		}
		if offset > 0 {
			db = db.Offset(offset)
		}
		return db
	}
}

// WithStatusFilter applies a status filter only if the status is non-empty.
func WithStatusFilter(status string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if status != "" {
			return db.Where("status = ?", status)
		}
		return db
	}
}

// WithStatuses filters by multiple statuses using IN clause.
// If the slice is empty, no filter is applied.
func WithStatuses(statuses []string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if len(statuses) > 0 {
			return db.Where("status IN ?", statuses)
		}
		return db
	}
}

// WithTypeFilter applies a type filter only if the type is non-empty.
func WithTypeFilter(t string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if t != "" {
			return db.Where("type = ?", t)
		}
		return db
	}
}

// WithTypes filters by multiple types using IN clause.
func WithTypes(types []string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if len(types) > 0 {
			return db.Where("type IN ?", types)
		}
		return db
	}
}

// WithServerIDFilter applies a server_id filter only if non-empty.
func WithServerIDFilter(serverID string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if serverID != "" {
			return db.Where("server_id = ?", serverID)
		}
		return db
	}
}

// WithSiteIDFilter applies a site_id filter only if non-empty.
func WithSiteIDFilter(siteID string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if siteID != "" {
			return db.Where("site_id = ?", siteID)
		}
		return db
	}
}

// WithTeamIDFilter applies a team_id filter only if non-empty.
func WithTeamIDFilter(teamID string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if teamID != "" {
			return db.Where("team_id = ?", teamID)
		}
		return db
	}
}

// WithUserIDFilter applies a user_id filter only if non-empty.
func WithUserIDFilter(userID string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if userID != "" {
			return db.Where("user_id = ?", userID)
		}
		return db
	}
}

// WithNameFilter applies a name filter only if non-empty.
func WithNameFilter(name string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if name != "" {
			return db.Where("name = ?", name)
		}
		return db
	}
}

// WithNameLike applies a LIKE filter on name if non-empty.
func WithNameLike(pattern string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if pattern != "" {
			return db.Where("name LIKE ?", "%"+pattern+"%")
		}
		return db
	}
}

// WithSearch applies a LIKE search on the specified column.
func WithSearch(column, search string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if search != "" {
			return db.Where(column+" LIKE ?", "%"+search+"%")
		}
		return db
	}
}

// WithIDsFilter filters by a list of IDs using IN clause.
func WithIDsFilter(ids []string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if len(ids) > 0 {
			return db.Where("id IN ?", ids)
		}
		return db
	}
}

// WithColumnValue applies an equality filter on any column.
func WithColumnValue(column string, value interface{}) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if value != nil && value != "" {
			return db.Where(column+" = ?", value)
		}
		return db
	}
}

// WithColumnIn applies an IN filter on any column.
func WithColumnIn(column string, values interface{}) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(column+" IN ?", values)
	}
}

// WithColumnNotIn applies a NOT IN filter on any column.
func WithColumnNotIn(column string, values interface{}) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(column+" NOT IN ?", values)
	}
}

// WithColumnNull filters for NULL values in the specified column.
func WithColumnNull(column string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(column + " IS NULL")
	}
}

// WithColumnNotNull filters for non-NULL values in the specified column.
func WithColumnNotNull(column string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(column + " IS NOT NULL")
	}
}

// WithColumnBetween applies a BETWEEN filter on any column.
func WithColumnBetween(column string, from, to interface{}) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(column+" BETWEEN ? AND ?", from, to)
	}
}

// WithOrder applies an ORDER BY clause.
func WithOrder(column, direction string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if column != "" {
			return db.Order(column + " " + direction)
		}
		return db
	}
}

// WithOrderDesc applies an ORDER BY DESC clause.
func WithOrderDesc(column string) FilterOption {
	return WithOrder(column, "DESC")
}

// WithOrderAsc applies an ORDER BY ASC clause.
func WithOrderAsc(column string) FilterOption {
	return WithOrder(column, "ASC")
}

// WithPreload adds a preload for the specified relation.
func WithPreload(relation string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Preload(relation)
	}
}

// WithMultiplePreloads adds preloads for multiple relations.
// Note: Use this instead of WithPreloads from preloads.go when working with FilterOption.
func WithMultiplePreloads(relations ...string) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		for _, rel := range relations {
			db = db.Preload(rel)
		}
		return db
	}
}

// WithSoftDeleted includes soft-deleted records.
func WithSoftDeleted() FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Unscoped()
	}
}

// OnlyActive filters for records that are not archived.
func OnlyActive() FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("archived_at IS NULL")
	}
}

// OnlyArchived filters for records that are archived.
func OnlyArchived() FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("archived_at IS NOT NULL")
	}
}

// ConditionalFilter applies a filter only if the condition is true.
func ConditionalFilter(condition bool, opt FilterOption) FilterOption {
	return func(db *gorm.DB) *gorm.DB {
		if condition {
			return opt(db)
		}
		return db
	}
}
