package repositories

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/managedservice/contracts"
)

// Registry holds all managed-service module repositories.
type Registry struct {
	db             *gorm.DB
	managedService *ManagedServiceRepository
}

// NewRegistry creates a new managed-service repository registry.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:             db,
		managedService: NewManagedServiceRepository(db),
	}
}

// ManagedService returns the managed-service repository.
func (r *Registry) ManagedService() contracts.ManagedServiceRepository {
	return r.managedService
}

// DB returns the underlying database connection.
func (r *Registry) DB() *gorm.DB { return r.db }
