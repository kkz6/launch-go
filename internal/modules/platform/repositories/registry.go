package repositories

import (
	"gorm.io/gorm"
)

// Registry holds all platform module repositories
type Registry struct {
	db                   *gorm.DB
	platformUpdate       *PlatformUpdateRepository
	serverPlatformUpdate *ServerPlatformUpdateRepository
	dismissal            *DismissalRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:                   db,
		platformUpdate:       NewPlatformUpdateRepository(db),
		serverPlatformUpdate: NewServerPlatformUpdateRepository(db),
		dismissal:            NewDismissalRepository(db),
	}
}

// PlatformUpdate returns the platform update repository
func (r *Registry) PlatformUpdate() *PlatformUpdateRepository { return r.platformUpdate }

// ServerPlatformUpdate returns the server platform update repository
func (r *Registry) ServerPlatformUpdate() *ServerPlatformUpdateRepository {
	return r.serverPlatformUpdate
}

// Dismissal returns the dismissal repository
func (r *Registry) Dismissal() *DismissalRepository { return r.dismissal }

// DB returns the underlying database connection
func (r *Registry) DB() *gorm.DB { return r.db }
