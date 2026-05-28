package services

import (
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ServiceDeps holds the dependencies for docker-module services.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]
	ServerRepos *serverrepos.Registry
	// BackupRepos exposes the global storage_providers repository so
	// docker database backups can reference a saved provider instead of
	// embedding S3 credentials per database. nil-tolerant — if a wiring
	// stage hasn't populated it, BackupService falls back to a clear
	// "storage providers unavailable" error.
	BackupRepos *backuprepos.Registry
	// Notifier dispatches email / Slack / Discord / Telegram messages
	// for backup success/failure (driven by the per-config
	// NotifyOnSuccess / NotifyOnFailure flags). nil-tolerant: missing
	// notifier is silently no-op so test rigs and partial bring-ups
	// don't crash on the synchronous RunNow path.
	Notifier taskrunner.NotifierService
	// AppURL is the externally-reachable base URL the GHA workflow
	// templates embed in their `curl` notify calls. Read from
	// APP_URL config at module boot. Empty in dev rigs that don't
	// wire it — the gha:bootstrap_workflow job will surface that as
	// a "workflow file has empty LAUNCH_WEBHOOK_URL variable" error
	// rather than silently committing something broken.
	AppURL string
}

// BaseService is the shared dependency carrier for every docker service.
type BaseService struct {
	*service.ModuleBase[*repositories.Registry]
	serviceDeps *ServiceDeps
}

// NewBaseService builds the carrier from typed deps.
func NewBaseService(deps *ServiceDeps) *BaseService {
	return &BaseService{
		ModuleBase:  service.NewModuleBase(&deps.ModuleDeps),
		serviceDeps: deps,
	}
}

// ServiceDeps returns the typed dependencies.
func (s *BaseService) ServiceDeps() *ServiceDeps { return s.serviceDeps }

// ServerRepos returns the server module's repository registry — used for
// cross-module checks (e.g. "does this server exist and belong to the
// caller's team?").
func (s *BaseService) ServerRepos() *serverrepos.Registry {
	return s.serviceDeps.ServerRepos
}

// BackupRepos returns the backup module's repository registry, used by
// BackupService to look up the saved storage_provider rows that docker
// database backups reference.
func (s *BaseService) BackupRepos() *backuprepos.Registry {
	return s.serviceDeps.BackupRepos
}

// Notifier returns the configured notifier service (email/slack/etc).
// May be nil if the host didn't wire one — callers should nil-check
// before dispatching so missing notifier infra never panics RunNow.
func (s *BaseService) Notifier() taskrunner.NotifierService {
	return s.serviceDeps.Notifier
}

// AppURL returns the externally-reachable base URL. See ServiceDeps
// for why this matters to GHA-build flows.
func (s *BaseService) AppURL() string {
	return s.serviceDeps.AppURL
}
