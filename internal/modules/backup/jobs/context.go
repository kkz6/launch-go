package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// BackupRepos combines backup and server repositories for the backup module.
type BackupRepos struct {
	registry *repositories.Registry
	server   servercontracts.RepositoryRegistry
}

// Backup returns the backup repository.
func (r *BackupRepos) Backup() *repositories.BackupRepository {
	return r.registry.Backup()
}

// BackupJob returns the backup job repository.
func (r *BackupRepos) BackupJob() *repositories.BackupJobRepository {
	return r.registry.BackupJob()
}

// StorageProvider returns the storage provider repository.
func (r *BackupRepos) StorageProvider() *repositories.StorageProviderRepository {
	return r.registry.StorageProvider()
}

// Server returns the server repository registry.
func (r *BackupRepos) Server() servercontracts.RepositoryRegistry {
	return r.server
}

// JobContext provides shared dependencies for all backup jobs.
// It embeds pkgjobs.ModuleContext for common functionality and typed repository access.
//
// Access common dependencies via inherited methods:
//   - ctx.DB() - database connection
//   - ctx.Logger() - zerolog logger
//   - ctx.Queue() - queue client
//   - ctx.Repos() - repository registry (*BackupRepos)
type JobContext struct {
	*pkgjobs.ModuleContext[*BackupRepos]
}

// NewJobContext creates a new job context.
func NewJobContext(
	db *gorm.DB,
	repos *repositories.Registry,
	serverRepos servercontracts.RepositoryRegistry,
	logger *zerolog.Logger,
	queueClient *queue.Client,
) *JobContext {
	backupRepos := &BackupRepos{
		registry: repos,
		server:   serverRepos,
	}

	return &JobContext{
		ModuleContext: pkgjobs.NewModuleContext(pkgjobs.BaseDeps{
			DB:     db,
			Logger: logger,
			Queue:  queueClient,
		}, backupRepos),
	}
}
