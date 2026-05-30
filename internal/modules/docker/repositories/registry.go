package repositories

import "gorm.io/gorm"

// Registry holds all docker-module repositories.
type Registry struct {
	project            *ProjectRepository
	application        *ApplicationRepository
	compose            *ComposeRepository
	database           *DatabaseRepository
	deployment         *DeploymentRepository
	domain             *DomainRepository
	envVar             *EnvVarRepository
	projectEnvVar      *ProjectEnvVarRepository
	databaseEnvVar     *DatabaseEnvVarRepository
	buildSecret        *BuildSecretRepository
	composeBuildSecret *ComposeBuildSecretRepository
	volume             *VolumeRepository
	schedule           *ScheduleRepository
	backup             *BackupRepository
	backupRun          *BackupRunRepository
	registryCred       *RegistryCredentialRepository
}

// NewRegistry wires up the repositories.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		project:            NewProjectRepository(db),
		application:        NewApplicationRepository(db),
		compose:            NewComposeRepository(db),
		database:           NewDatabaseRepository(db),
		deployment:         NewDeploymentRepository(db),
		domain:             NewDomainRepository(db),
		envVar:             NewEnvVarRepository(db),
		projectEnvVar:      NewProjectEnvVarRepository(db),
		databaseEnvVar:     NewDatabaseEnvVarRepository(db),
		buildSecret:        NewBuildSecretRepository(db),
		composeBuildSecret: NewComposeBuildSecretRepository(db),
		volume:             NewVolumeRepository(db),
		schedule:           NewScheduleRepository(db),
		backup:             NewBackupRepository(db),
		backupRun:          NewBackupRunRepository(db),
		registryCred:       NewRegistryCredentialRepository(db),
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

// ProjectEnvVar returns the project-scoped env-var repository — the
// source for `${{project.<KEY>}}` references resolved at deploy/run
// time.
func (r *Registry) ProjectEnvVar() *ProjectEnvVarRepository { return r.projectEnvVar }

// DatabaseEnvVar returns the env-var repository scoped to a managed
// database (user-added extras on top of the auto-generated engine
// credentials).
func (r *Registry) DatabaseEnvVar() *DatabaseEnvVarRepository { return r.databaseEnvVar }

// BuildSecret returns the application build-secret repository — name/
// value pairs mounted into `docker build` via BuildKit's
// --mount=type=secret. NOT visible to `docker run`; for runtime use
// EnvVar instead.
func (r *Registry) BuildSecret() *BuildSecretRepository { return r.buildSecret }

// ComposeBuildSecret returns the compose-stack mirror of BuildSecret.
// One secret name is available to every service in the stack that
// references it from its Dockerfile.
func (r *Registry) ComposeBuildSecret() *ComposeBuildSecretRepository {
	return r.composeBuildSecret
}

// Volume returns the docker volume repository — polymorphic by
// owner. The same row type backs application volumes AND compose-
// stack volumes; the repo exposes per-owner List / ExistsByName
// methods that filter by application_id or compose_id.
func (r *Registry) Volume() *VolumeRepository { return r.volume }

// Schedule returns the application schedule repository.
func (r *Registry) Schedule() *ScheduleRepository { return r.schedule }

// Backup returns the database backup-config repository.
func (r *Registry) Backup() *BackupRepository { return r.backup }

// BackupRun returns the database backup-run history repository.
func (r *Registry) BackupRun() *BackupRunRepository { return r.backupRun }

// RegistryCredential returns the docker-registry credential
// repository — the team-scoped saved-login table that backs
// the application "saved credential" picker + compose stack
// many-to-many auth attachments.
func (r *Registry) RegistryCredential() *RegistryCredentialRepository {
	return r.registryCred
}
