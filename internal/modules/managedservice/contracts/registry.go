package contracts

// RepositoryRegistry exposes the managed-service repositories used by
// services and jobs.
type RepositoryRegistry interface {
	ManagedService() ManagedServiceRepository
}
