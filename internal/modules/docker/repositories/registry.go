package repositories

import "gorm.io/gorm"

// Registry holds all docker-module repositories.
type Registry struct {
	project *ProjectRepository
}

// NewRegistry wires up the repositories.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		project: NewProjectRepository(db),
	}
}

// Project returns the project repository.
func (r *Registry) Project() *ProjectRepository { return r.project }
