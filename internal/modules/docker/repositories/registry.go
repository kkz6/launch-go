package repositories

import "gorm.io/gorm"

// Registry holds all docker-module repositories.
type Registry struct {
	project     *ProjectRepository
	application *ApplicationRepository
	deployment  *DeploymentRepository
}

// NewRegistry wires up the repositories.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		project:     NewProjectRepository(db),
		application: NewApplicationRepository(db),
		deployment:  NewDeploymentRepository(db),
	}
}

// Project returns the project repository.
func (r *Registry) Project() *ProjectRepository { return r.project }

// Application returns the application repository.
func (r *Registry) Application() *ApplicationRepository { return r.application }

// Deployment returns the deployment repository.
func (r *Registry) Deployment() *DeploymentRepository { return r.deployment }
