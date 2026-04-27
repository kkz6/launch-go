package repositories

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dockerregistry/contracts"
)

// Registry holds all docker-registry module repositories.
type Registry struct {
	db         *gorm.DB
	credential *CredentialRepository
}

// NewRegistry creates the registry.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:         db,
		credential: NewCredentialRepository(db),
	}
}

// Credential returns the credential repository.
func (r *Registry) Credential() contracts.CredentialRepository { return r.credential }

// DB returns the underlying database connection.
func (r *Registry) DB() *gorm.DB { return r.db }
