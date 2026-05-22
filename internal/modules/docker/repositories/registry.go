package repositories

import "gorm.io/gorm"

// Registry holds all docker-module repositories.
type Registry struct {
	project     *ProjectRepository
	application *ApplicationRepository
	compose     *ComposeRepository
	deployment  *DeploymentRepository
	domain      *DomainRepository
}

// NewRegistry wires up the repositories.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		project:     NewProjectRepository(db),
		application: NewApplicationRepository(db),
		compose:     NewComposeRepository(db),
		deployment:  NewDeploymentRepository(db),
		domain:      NewDomainRepository(db),
	}
}

// Project returns the project repository.
func (r *Registry) Project() *ProjectRepository { return r.project }

// Application returns the application repository.
func (r *Registry) Application() *ApplicationRepository { return r.application }

// Compose returns the compose-stack repository.
func (r *Registry) Compose() *ComposeRepository { return r.compose }

// Deployment returns the deployment repository.
func (r *Registry) Deployment() *DeploymentRepository { return r.deployment }

// Domain returns the application domain repository.
func (r *Registry) Domain() *DomainRepository { return r.domain }
