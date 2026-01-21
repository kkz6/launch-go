package repository

import "gorm.io/gorm"

// WithPreloads applies a list of preloads to a query.
// Use this for repositories that need consistent preloading across multiple methods.
//
// Usage:
//
//	query := repository.WithPreloads(r.DB.WithContext(ctx), []string{"Users", "Server"})
//	err := query.First(&entity, "id = ?", id).Error
func WithPreloads(query *gorm.DB, preloads []string) *gorm.DB {
	for _, p := range preloads {
		query = query.Preload(p)
	}
	return query
}

// PreloadScope returns a scope function that applies the given preloads.
// Useful for composing with other scopes.
//
// Usage:
//
//	scope := repository.PreloadScope("Users", "Server", "Sites")
//	query := db.Scopes(scope)
func PreloadScope(preloads ...string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		for _, p := range preloads {
			db = db.Preload(p)
		}
		return db
	}
}

// PreloadConfig holds default preload configuration for a repository.
// Embed this in repositories that need consistent preloading.
//
// Usage:
//
//	type DatabaseRepository struct {
//	    repository.Base[models.Database]
//	    repository.PreloadConfig
//	}
//
//	func NewDatabaseRepository(db *gorm.DB) *DatabaseRepository {
//	    return &DatabaseRepository{
//	        Base:          repository.NewBase[models.Database](db),
//	        PreloadConfig: repository.NewPreloadConfig("Users"),
//	    }
//	}
type PreloadConfig struct {
	defaultPreloads []string
}

// NewPreloadConfig creates a new PreloadConfig with the given preloads.
func NewPreloadConfig(preloads ...string) PreloadConfig {
	return PreloadConfig{defaultPreloads: preloads}
}

// DefaultPreloads returns the default preloads for this config.
func (c PreloadConfig) DefaultPreloads() []string {
	return c.defaultPreloads
}

// ApplyPreloads applies the default preloads to a query.
func (c PreloadConfig) ApplyPreloads(query *gorm.DB) *gorm.DB {
	return WithPreloads(query, c.defaultPreloads)
}

// PreloadScopeFromConfig returns a scope function using the config's preloads.
func (c PreloadConfig) PreloadScopeFromConfig() func(*gorm.DB) *gorm.DB {
	return PreloadScope(c.defaultPreloads...)
}
