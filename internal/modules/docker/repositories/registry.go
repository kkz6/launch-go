package repositories

import "gorm.io/gorm"

// Registry holds all docker-module repositories.
type Registry struct {
	project     *ProjectRepository
	application *ApplicationRepository
	compose     *ComposeRepository
	database    *DatabaseRepository
	deployment  *DeploymentRepository
	domain      *DomainRepository
	envVar      *EnvVarRepository
	volume      *VolumeRepository
	schedule    *ScheduleRepository
}

// NewRegistry wires up the repositories.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		project:     NewProjectRepository(db),
		application: NewApplicationRepository(db),
		compose:     NewComposeRepository(db),
		database:    NewDatabaseRepository(db),
		deployment:  NewDeploymentRepository(db),
		domain:      NewDomainRepository(db),
		envVar:      NewEnvVarRepository(db),
		volume:      NewVolumeRepository(db),
		schedule:    NewScheduleRepository(db),
	}
}

// Project returns the project repository.
func (r *Registry) Project() *ProjectRepository { return r.project }

// Application returns the application repository.
func (r *Registry) Application() *ApplicationRepository { return r.application }

// Compose returns the compose-stack repository.
func (r *Registry) Compose() *ComposeRepository { return r.compose }

// Database returns the managed-database repository.
func (r *Registry) Database() *DatabaseRepository { return r.database }

// Deployment returns the deployment repository.
func (r *Registry) Deployment() *DeploymentRepository { return r.deployment }

// Domain returns the application domain repository.
func (r *Registry) Domain() *DomainRepository { return r.domain }

// EnvVar returns the application env-var repository.
func (r *Registry) EnvVar() *EnvVarRepository { return r.envVar }

// Volume returns the application volume repository.
func (r *Registry) Volume() *VolumeRepository { return r.volume }

// Schedule returns the application schedule repository.
func (r *Registry) Schedule() *ScheduleRepository { return r.schedule }
