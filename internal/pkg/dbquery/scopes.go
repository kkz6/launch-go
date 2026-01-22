// Package dbquery provides GORM query helpers and scope functions.
package dbquery

import "gorm.io/gorm"

// ByTeam returns a GORM scope function that filters by team ID
func ByTeam(teamID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("team_id = ?", teamID)
	}
}

// ByServer returns a GORM scope function that filters by server ID
func ByServer(serverID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("server_id = ?", serverID)
	}
}

// BySite returns a GORM scope function that filters by site ID
func BySite(siteID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("site_id = ?", siteID)
	}
}

// ByUser returns a GORM scope function that filters by user ID
func ByUser(userID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("user_id = ?", userID)
	}
}

// NotArchived returns a GORM scope function that excludes archived records
func NotArchived() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("archived_at IS NULL")
	}
}

// Archived returns a GORM scope function that only includes archived records
func Archived() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("archived_at IS NOT NULL")
	}
}

// ByStatus returns a GORM scope function that filters by status
func ByStatus[S ~string](status S) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", status)
	}
}

// ByStatusIn returns a GORM scope function that filters by multiple statuses
func ByStatusIn[S ~string](statuses ...S) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status IN ?", statuses)
	}
}

// ByID returns a GORM scope function that filters by ID
func ByID(id string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id)
	}
}

// ByIDs returns a GORM scope function that filters by multiple IDs
func ByIDs(ids ...string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("id IN ?", ids)
	}
}
