// Package docker is the docker-workloads module: projects, applications,
// compose stacks, and managed databases that live on a docker-type server.
//
// Phase 1 ships projects CRUD only. Applications/Compose/Databases are
// modelled (so foreign keys land cleanly) but their services and worker
// pipelines come in later phases — see
// docs/plans/2026-05-22-docker-server-menus-design.md.
package docker

import (
	"github.com/hibiken/asynq"

	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	certrepos "github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/services"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// ModuleName is the registered name for this module.
const ModuleName = "docker"

var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module wires the docker module into the framework.
type Module struct {
	app.Base
	repos       *repositories.Registry
	serverRepos *serverrepos.Registry
	backupRepos *backuprepos.Registry
	// certRepos exposes the team-scoped certificate library so the
	// Traefik sync job can resolve stored cert PEMs and ship them to
	// the docker server. Injected at app boot via
	// SetCertificateRepository (cross-module — owned by the
	// certificate module).
	certRepos *certrepos.Registry
}

// NewModule constructs the module. ServerRepos + BackupRepos are
// created here (not injected) so the module owns its dependency graph;
// BackupRepos is what lets database backups reference a saved storage
// provider instead of embedding S3 creds per row.
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	return &Module{
		Base:        app.NewBase(ModuleName, b),
		repos:       repositories.NewRegistry(deps.DB),
		serverRepos: serverrepos.NewRegistry(deps.DB),
		backupRepos: backuprepos.NewRegistry(deps.DB),
	}
}

// Repos exposes the repository registry for cross-module access.
func (m *Module) Repos() *repositories.Registry { return m.repos }

// SetCertificateRepository injects the certificate module's repository
// so SyncTraefikConfig can resolve PEMs for stored-cert domains.
// Called once at boot from cmd/api/main.go.
func (m *Module) SetCertificateRepository(r *certrepos.Registry) {
	m.certRepos = r
}

// RegisterJobs implements app.JobRegistrar. Binds every docker asynq task
// type to its handler.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos, m.serverRepos, m.backupRepos, m.certRepos)
}

// newProjectService builds the project service once per request boot.
func (m *Module) newProjectService() *services.ProjectService {
	return services.NewProjectService(m.serviceDeps())
}

// newApplicationService builds the application service.
func (m *Module) newApplicationService() *services.ApplicationService {
	return services.NewApplicationService(m.serviceDeps())
}

// newComposeService builds the compose service.
func (m *Module) newComposeService() *services.ComposeService {
	return services.NewComposeService(m.serviceDeps())
}

// newDatabaseService builds the managed-database service.
func (m *Module) newDatabaseService() *services.DatabaseService {
	return services.NewDatabaseService(m.serviceDeps())
}

// newDomainService builds the application-domain service.
func (m *Module) newDomainService() *services.DomainService {
	return services.NewDomainService(m.serviceDeps())
}

// newEnvVarService builds the application env-var service.
func (m *Module) newEnvVarService() *services.EnvVarService {
	return services.NewEnvVarService(m.serviceDeps())
}

// newProjectEnvVarService builds the project-scoped env-var service —
// the source for `${{project.<KEY>}}` references that workload env
// vars resolve against at deploy/run time.
func (m *Module) newProjectEnvVarService() *services.ProjectEnvVarService {
	return services.NewProjectEnvVarService(m.serviceDeps())
}

// newDatabaseEnvVarService builds the database env-var service —
// user-added extras layered on top of the auto-generated engine
// credentials.
func (m *Module) newDatabaseEnvVarService() *services.DatabaseEnvVarService {
	return services.NewDatabaseEnvVarService(m.serviceDeps())
}

// newVolumeService builds the application volume service.
func (m *Module) newVolumeService() *services.VolumeService {
	return services.NewVolumeService(m.serviceDeps())
}

// newHostInspectService builds the docker-host diagnostic service used
// by the Containers/Volumes/Networks/Traefik tabs.
func (m *Module) newHostInspectService() *services.HostInspectService {
	return services.NewHostInspectService(m.serviceDeps())
}

// newScheduleService builds the application schedule service.
func (m *Module) newScheduleService() *services.ScheduleService {
	return services.NewScheduleService(m.serviceDeps())
}

// newBackupService builds the database-backup service.
func (m *Module) newBackupService() *services.BackupService {
	return services.NewBackupService(m.serviceDeps())
}

// newRegistryCredentialService builds the docker-registry credential
// service backing the Settings → Connections CRUD + the
// application / compose registry pickers.
func (m *Module) newRegistryCredentialService() *services.RegistryCredentialService {
	return services.NewRegistryCredentialService(m.serviceDeps())
}

func (m *Module) serviceDeps() *services.ServiceDeps {
	deps := m.Deps()
	return &services.ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: deps.ServiceDeps(),
			Repos:        m.repos,
		},
		ServerRepos: m.serverRepos,
		BackupRepos: m.backupRepos,
		// Notifier is shared with the worker side (TaskRunnerDeps.
		// Notifier). The API path uses it from BackupService.RunNow's
		// post-completion dispatch so manual runs honour
		// NotifyOnSuccess / NotifyOnFailure the same as scheduled
		// runs do — same notification structs, same channel routing.
		Notifier: deps.Notifier,
	}
}
