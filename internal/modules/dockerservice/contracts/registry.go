package contracts

// RepositoryRegistry exposes the docker-service repositories used by
// services and jobs.
type RepositoryRegistry interface {
	DockerService() DockerServiceRepository
}
