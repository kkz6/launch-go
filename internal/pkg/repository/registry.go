package repository

import "gorm.io/gorm"

// RegistryBase provides common registry functionality for module repository registries.
// Modules should embed this struct to get common DB access functionality.
//
// Example usage:
//
//	type Registry struct {
//	    repository.RegistryBase
//	    user *UserRepository
//	    team *TeamRepository
//	}
//
//	func NewRegistry(db *gorm.DB) *Registry {
//	    return &Registry{
//	        RegistryBase: repository.NewRegistryBase(db),
//	        user:         NewUserRepository(db),
//	        team:         NewTeamRepository(db),
//	    }
//	}
type RegistryBase struct {
	db *gorm.DB
}

// NewRegistryBase creates a new RegistryBase instance
func NewRegistryBase(db *gorm.DB) RegistryBase {
	return RegistryBase{db: db}
}

// DB returns the underlying database connection
func (r RegistryBase) DB() *gorm.DB {
	return r.db
}

// WithTransaction executes the given function within a database transaction
func (r RegistryBase) WithTransaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// RepositoryBase provides common functionality for individual repositories.
// Repositories should embed this struct to get common DB access functionality.
//
// Example usage:
//
//	type UserRepository struct {
//	    repository.RepositoryBase
//	}
//
//	func NewUserRepository(db *gorm.DB) *UserRepository {
//	    return &UserRepository{
//	        RepositoryBase: repository.NewRepositoryBase(db),
//	    }
//	}
type RepositoryBase struct {
	db *gorm.DB
}

// NewRepositoryBase creates a new RepositoryBase instance
func NewRepositoryBase(db *gorm.DB) RepositoryBase {
	return RepositoryBase{db: db}
}

// DB returns the underlying database connection
func (r RepositoryBase) DB() *gorm.DB {
	return r.db
}
