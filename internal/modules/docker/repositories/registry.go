package repositories

import "gorm.io/gorm"

// Registry holds all docker module repositories
type Registry struct {
	db               *gorm.DB
	dockerService    *DockerServiceRepository
	dockerDeployment *DockerDeploymentRepository
	dockerRegistry   *DockerRegistryRepository
	envVar           *DockerEnvVarRepository
	volume           *DockerVolumeRepository
	port             *DockerPortRepository
	domain           *DockerDomainRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:               db,
		dockerService:    NewDockerServiceRepository(db),
		dockerDeployment: NewDockerDeploymentRepository(db),
		dockerRegistry:   NewDockerRegistryRepository(db),
		envVar:           NewDockerEnvVarRepository(db),
		volume:           NewDockerVolumeRepository(db),
		port:             NewDockerPortRepository(db),
		domain:           NewDockerDomainRepository(db),
	}
}

// DockerService returns the docker service repository
func (r *Registry) DockerService() *DockerServiceRepository { return r.dockerService }

// DockerDeployment returns the docker deployment repository
func (r *Registry) DockerDeployment() *DockerDeploymentRepository { return r.dockerDeployment }

// DockerRegistry returns the docker registry repository
func (r *Registry) DockerRegistry() *DockerRegistryRepository { return r.dockerRegistry }

// EnvVar returns the docker env var repository
func (r *Registry) EnvVar() *DockerEnvVarRepository { return r.envVar }

// Volume returns the docker volume repository
func (r *Registry) Volume() *DockerVolumeRepository { return r.volume }

// Port returns the docker port repository
func (r *Registry) Port() *DockerPortRepository { return r.port }

// Domain returns the docker domain repository
func (r *Registry) Domain() *DockerDomainRepository { return r.domain }

// DB returns the underlying database connection
func (r *Registry) DB() *gorm.DB { return r.db }
