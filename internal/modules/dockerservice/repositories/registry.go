package repositories

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dockerservice/contracts"
)

// Registry holds all docker-service module repositories.
type Registry struct {
	db            *gorm.DB
	dockerService *DockerServiceRepository
}

// NewRegistry creates a new docker-service repository registry.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:            db,
		dockerService: NewDockerServiceRepository(db),
	}
}

// DockerService returns the docker-service repository.
func (r *Registry) DockerService() contracts.DockerServiceRepository {
	return r.dockerService
}

// DB returns the underlying database connection.
func (r *Registry) DB() *gorm.DB { return r.db }
