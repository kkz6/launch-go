package dto

// CreateProjectRequest is the request body for creating a docker project.
type CreateProjectRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
}

// UpdateProjectRequest is the partial-update body for a project.
// Both fields are optional; only present keys are applied.
type UpdateProjectRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
}

// CreateApplicationRequest is the request body for registering an application
// inside a project. The source_type field is the discriminator that selects
// which of {ImageSource, GitSource, DockerfileSource} the rest of the payload
// will populate.
//
// We accept all three source-type carriers in one request struct (each
// nullable) rather than three separate endpoints — the validator below
// enforces "exactly one is present for the matching source_type".
type CreateApplicationRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`

	// InternalPort is the port the container listens on internally. Traefik
	// routes here. Optional; default 80 if unset. Validated in the service.
	InternalPort *int `json:"internal_port,omitempty" validate:"omitempty,min=1,max=65535"`

	SourceType string `json:"source_type" validate:"required,oneof=image git dockerfile"`

	// image source: one of these payloads must be set when source_type=image.
	// `dive` is NOT valid on struct pointers — validator descends into nested
	// structs automatically, and using dive here panics deep inside
	// traverseField. The discriminator (source_type) + service-level checks
	// ensure exactly one payload is present.
	Image *ImageSourceInput `json:"image,omitempty"`

	// git source.
	Git *GitSourceInput `json:"git,omitempty"`

	// dockerfile source (user-pasted Dockerfile content).
	Dockerfile *DockerfileSourceInput `json:"dockerfile,omitempty"`
}

// ImageSourceInput carries the info needed to pull and run a pre-built image.
type ImageSourceInput struct {
	// Image is the full reference including registry/repo/tag, e.g.
	// "nginx:1.27" or "ghcr.io/acme/api:v3". Tag is required — we don't
	// silently default to :latest because that hides upgrades from the user.
	Image string `json:"image" validate:"required,min=1,max=512"`
	// RegistryCredentialID optionally points at a private-registry credential
	// the user has connected. Left empty for public images.
	RegistryCredentialID *string `json:"registry_credential_id,omitempty"`
}

// GitSourceInput carries the info needed to clone + build from a git repo.
type GitSourceInput struct {
	// Repo is the clone URL — https://github.com/owner/repo or an ssh remote.
	Repo   string `json:"repo" validate:"required,min=1,max=512"`
	Branch string `json:"branch" validate:"required,min=1,max=255"`
	// SourceControlID, when set, picks the private-repo connection used to
	// authenticate the clone. Left empty for public repos.
	SourceControlID *string `json:"source_control_id,omitempty"`
	// BuildType is "nixpacks" | "dockerfile". If omitted the deploy job
	// auto-detects (Dockerfile in repo root → dockerfile, else nixpacks).
	BuildType *string `json:"build_type,omitempty" validate:"omitempty,oneof=nixpacks dockerfile"`
	// DockerfilePath, when BuildType=dockerfile, overrides the default
	// `./Dockerfile`. Useful for monorepos.
	DockerfilePath *string `json:"dockerfile_path,omitempty" validate:"omitempty,max=512"`
}

// DockerfileSourceInput carries a raw Dockerfile pasted into the UI.
type DockerfileSourceInput struct {
	// Contents is the full Dockerfile body. Capped at 64 KiB so a malicious
	// or accidental dump can't gum up the storage layer.
	Contents string `json:"contents" validate:"required,min=1,max=65536"`
}

// UpdateApplicationRequest is the partial-update body for an application.
// Only Name is mutable in phase 2a — source/build settings are immutable
// until later slices add a "reconfigure" flow.
type UpdateApplicationRequest struct {
	Name *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
}

// CreateComposeRequest registers a docker-compose stack inside a project.
// The compose_source_type discriminates which payload (git or raw_yaml)
// is required — the service validates that exactly one is present.
type CreateComposeRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`

	// ComposeSourceType is "git" or "raw_yaml". Different from the
	// application source type set — compose doesn't have "image" or
	// "dockerfile" branches because the YAML defines services itself.
	ComposeSourceType string `json:"compose_source_type" validate:"required,oneof=git raw_yaml"`

	// Git is present when compose_source_type=git.
	//
	// No `dive` tag: go-playground/validator's `dive` traverses
	// slices/maps/arrays — applied to a struct pointer it panics
	// (same crash hit on CreateApplicationRequest earlier). The
	// validator recurses into struct-pointer fields automatically,
	// honouring nested tags on ComposeGitInput's own fields, so the
	// tag is unnecessary as well as harmful.
	Git *ComposeGitInput `json:"git,omitempty"`

	// RawYAML is present when compose_source_type=raw_yaml. Same
	// reasoning as Git above — no `dive` on the struct pointer.
	RawYAML *ComposeRawYAMLInput `json:"raw_yaml,omitempty"`
}

// ComposeGitInput clones a repository containing a docker-compose file.
type ComposeGitInput struct {
	Repo            string  `json:"repo" validate:"required,min=1,max=512"`
	Branch          string  `json:"branch" validate:"required,min=1,max=255"`
	SourceControlID *string `json:"source_control_id,omitempty"`
	// ComposeFilePath defaults to docker-compose.yml if empty.
	ComposeFilePath *string `json:"compose_file_path,omitempty" validate:"omitempty,max=512"`
}

// ComposeRawYAMLInput stores the docker-compose YAML inline. Capped at
// 128 KiB — generous, but bounded so a copy-paste of a giant compose
// file doesn't bloat the row.
type ComposeRawYAMLInput struct {
	Contents string `json:"contents" validate:"required,min=1,max=131072"`
}

// UpdateComposeRequest is the partial-update body. Like
// UpdateApplicationRequest, only the name is mutable in phase 2h.
type UpdateComposeRequest struct {
	Name *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	// EnvFile is the `.env` body the compose Environment subtab
	// edits. Empty string is meaningful — clears the file. Use
	// `null`/omit to leave unchanged. Max 128 KiB matches the
	// raw_yaml cap so the row stays bounded.
	EnvFile *string `json:"env_file,omitempty" validate:"omitempty,max=131072"`
	// RunCommand replaces the docker command suffix the deploy
	// script uses. Empty string clears the override (deploy reverts
	// to the default `compose -p NAME -f FILE up -d --build
	// --remove-orphans`). Use `null`/omit to leave unchanged.
	// Bounded at 8 KiB — realistic ceiling is a few hundred chars,
	// the cap is just to keep a runaway payload from bloating the
	// row.
	RunCommand *string `json:"run_command,omitempty" validate:"omitempty,max=8192"`
}

// CreateEnvVarRequest adds a single env var to an application. Use
// SetEnvVarsRequest below to replace the entire set in one call
// (cleaner for the bulk-paste UI).
type CreateEnvVarRequest struct {
	Key      string `json:"key" validate:"required,min=1,max=255"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret,omitempty"`
}

// UpdateEnvVarRequest is the partial-update body. Key is immutable —
// removing + adding is the way to "rename" (otherwise running
// containers would silently keep the old key).
type UpdateEnvVarRequest struct {
	Value    *string `json:"value,omitempty"`
	IsSecret *bool   `json:"is_secret,omitempty"`
}

// SetEnvVarsRequest replaces the entire env-var set in one
// transaction. Matches the "paste your .env file" workflow — easier
// than asking the user to click N times.
type SetEnvVarsRequest struct {
	Vars []CreateEnvVarRequest `json:"vars" validate:"required,dive"`
}

// CreateVolumeRequest attaches a mount to an application. Type is
// the discriminator:
//
//   - bind   → host_path required (absolute path on the docker host)
//   - volume → name is the docker-named volume (legacy alias: "named")
//   - file   → content + file_path required; content is written to
//             <deploy_dir>/<file_path> on the host and bind-mounted
//             at mount_path inside the container.
type CreateVolumeRequest struct {
	Name      string  `json:"name" validate:"required,min=1,max=255"`
	MountPath string  `json:"mount_path" validate:"required,min=1,max=512"`
	Type      string  `json:"type" validate:"required,oneof=named volume bind file"`
	HostPath  *string `json:"host_path,omitempty" validate:"omitempty,max=512"`
	// File-mount payload — content gets persisted; file_path is the
	// filename written on the host. The deploy script writes
	// `<deploy_dir>/<file_path>` then bind-mounts that path at
	// mount_path inside the container.
	Content  *string `json:"content,omitempty"`
	FilePath *string `json:"file_path,omitempty" validate:"omitempty,max=512"`
}

// UpdateVolumeRequest is the partial-update body. Mount path can be
// changed but the volume "name" identifies the named-volume in docker
// — renaming would orphan the old one. For file mounts, content
// and/or file_path can be edited in place.
type UpdateVolumeRequest struct {
	MountPath *string `json:"mount_path,omitempty" validate:"omitempty,min=1,max=512"`
	HostPath  *string `json:"host_path,omitempty" validate:"omitempty,max=512"`
	Content   *string `json:"content,omitempty"`
	FilePath  *string `json:"file_path,omitempty" validate:"omitempty,max=512"`
}

// CreateDatabaseRequest provisions a managed-database container in a
// project. Engine is the discriminator; version defaults to the
// engine's pinned default if omitted.
type CreateDatabaseRequest struct {
	Name    string `json:"name" validate:"required,min=1,max=64"`
	Engine  string `json:"engine" validate:"required,oneof=postgres mysql mariadb redis mongo"`
	Version string `json:"version,omitempty" validate:"omitempty,max=32"`
	// ExternalPort, when set, exposes the database on the host so it's
	// reachable from outside docker. Leave nil for internal-only (the
	// default) so the DB stays on launch-network and only sibling
	// containers can reach it.
	ExternalPort *int `json:"external_port,omitempty" validate:"omitempty,min=1,max=65535"`
}

// DatabaseLifecycleRequest is the body for the /lifecycle endpoint. We
// take the action as a JSON field instead of a path segment so the same
// endpoint handles all three transitions consistently.
type DatabaseLifecycleRequest struct {
	Action string `json:"action" validate:"required,oneof=start stop restart"`
}

// UpdateDatabaseAdvancedRequest is the body for PATCH /:id/advanced —
// the Advanced subtab's runtime knobs. RestartPolicy is required so a
// blank submission can't accidentally remove it; the resource knobs
// are optional strings in docker's --cpus / --memory / --memory-reservation
// formats (e.g. "0.5", "512m", "1g"). Empty strings clear the knob.
type UpdateDatabaseAdvancedRequest struct {
	RestartPolicy     string `json:"restart_policy" validate:"required,oneof=no on-failure always unless-stopped"`
	CPULimit          string `json:"cpu_limit,omitempty" validate:"omitempty,max=16"`
	MemoryLimit       string `json:"memory_limit,omitempty" validate:"omitempty,max=16"`
	CPUReservation    string `json:"cpu_reservation,omitempty" validate:"omitempty,max=16"`
	MemoryReservation string `json:"memory_reservation,omitempty" validate:"omitempty,max=16"`
}

// SetDatabaseExposeRequest is the body for POST /:id/expose — toggles
// the external port mapping on/off. When Enabled=true and Port is nil
// the engine's default port is used. The change triggers a container
// recreate (RunDatabaseScript is idempotent — stops + rm's any prior
// container of the same name).
type SetDatabaseExposeRequest struct {
	Enabled bool `json:"enabled"`
	Port    *int `json:"port,omitempty" validate:"omitempty,min=1,max=65535"`
}

// ConfigureBackupRequest enables (or updates) scheduled backups for a
// managed database. The destination is a saved storage_providers row
// (configured once under Settings → Connections); we never accept raw
// S3 credentials on this endpoint anymore — that pattern duplicated
// secrets per-database and made cred rotation a nightmare.
type ConfigureBackupRequest struct {
	StorageProviderID uint64  `json:"storage_provider_id" validate:"required,min=1"`
	Path              *string `json:"path,omitempty" validate:"omitempty,max=255"`
	// Retention is the number of past run rows + remote objects to
	// keep. Defaults to 10 in the migration; min 1 here so the worker
	// always has something to prune.
	Retention       int     `json:"retention" validate:"min=1,max=1000"`
	NotifyOnSuccess bool    `json:"notify_on_success"`
	NotifyOnFailure bool    `json:"notify_on_failure"`
	CronSchedule    *string `json:"cron_schedule,omitempty" validate:"omitempty,max=64"`
	Enabled         bool    `json:"enabled"`
}

// RestoreBackupRequest identifies which past run to restore from.
type RestoreBackupRequest struct {
	RunID string `json:"run_id" validate:"required"`
}

// CreateScheduleRequest adds a cron-style task that runs a command
// inside the application's container. Enabled defaults to true on
// create; ShellType defaults to "sh" because it's the universally-
// present shell across alpine + minimal images.
type CreateScheduleRequest struct {
	Cron      string  `json:"cron" validate:"required,min=1,max=255"`
	Command   string  `json:"command" validate:"required,min=1"`
	Enabled   *bool   `json:"enabled,omitempty"`
	ShellType *string `json:"shell_type,omitempty" validate:"omitempty,oneof=bash sh"`
}

// UpdateScheduleRequest is the partial-update body. Any subset of
// fields can be sent; missing keys leave the existing value alone.
type UpdateScheduleRequest struct {
	Cron      *string `json:"cron,omitempty" validate:"omitempty,min=1,max=255"`
	Command   *string `json:"command,omitempty" validate:"omitempty,min=1"`
	Enabled   *bool   `json:"enabled,omitempty"`
	ShellType *string `json:"shell_type,omitempty" validate:"omitempty,oneof=bash sh"`
}

// UpdateApplicationTraefikConfigRequest carries the new YAML body
// for the per-application dynamic-config file. Size cap matches the
// server-level WriteTraefikDynamicFile path (256 KiB) so the writer
// can apply a single ceiling everywhere. Empty string is allowed —
// it overwrites the file with nothing, which is the right behaviour
// when the operator wants to disable an app's routes without
// removing the file via SSH.
type UpdateApplicationTraefikConfigRequest struct {
	Content string `json:"content" validate:"max=262144"`
}

// UpdateAdvancedRequest tweaks runtime knobs on an application without
// touching its source. All fields are optional; present keys are
// applied to the application's build_config and take effect on the
// next deploy.
type UpdateAdvancedRequest struct {
	// CPULimit in docker --cpus format (e.g. "0.5", "2"). Empty string clears.
	CPULimit *string `json:"cpu_limit,omitempty" validate:"omitempty,max=32"`
	// MemoryLimit in docker -m format (e.g. "512m", "2g"). Empty clears.
	MemoryLimit *string `json:"memory_limit,omitempty" validate:"omitempty,max=32"`
	// CPUReservation is the soft-floor for CPU shares (docker
	// --cpu-shares-ish; we pass it as --cpu-reservation via the deploy
	// stanza). Empty clears.
	CPUReservation *string `json:"cpu_reservation,omitempty" validate:"omitempty,max=32"`
	// MemoryReservation is the soft-floor in -m units. Empty clears.
	MemoryReservation *string `json:"memory_reservation,omitempty" validate:"omitempty,max=32"`
	// RestartPolicy: one of no / on-failure / always / unless-stopped.
	RestartPolicy *string `json:"restart_policy,omitempty" validate:"omitempty,oneof=no on-failure always unless-stopped"`
	// HealthcheckCommand is a single command run inside the container
	// for HEALTHCHECK. Empty clears the healthcheck.
	HealthcheckCommand *string `json:"healthcheck_command,omitempty" validate:"omitempty,max=512"`
	// ExtraPorts are host:container port mappings ("8080:80"). Each
	// entry is passed straight through to `docker run -p`.
	ExtraPorts []string `json:"extra_ports,omitempty" validate:"omitempty,dive,max=32"`
	// Redirects mirror dokploy's per-app redirect middleware. The
	// deploy job emits a Traefik regex-redirect middleware for each
	// entry on the next deploy. Nil = leave unchanged; empty slice =
	// clear all.
	Redirects *[]RedirectInput `json:"redirects,omitempty" validate:"omitempty,dive"`
	// Security is the single basic-auth credential applied via a
	// Traefik basicauth middleware. Nil = leave unchanged; pointer to
	// an empty struct = clear (no auth).
	Security *SecurityInput `json:"security,omitempty"`
}

// RedirectInput is kept for the legacy Advanced-card path (now
// deprecated in favour of the per-row CRUD endpoints under
// /applications/:id/redirects). New code should use
// CreateApplicationRedirectRequest instead.
type RedirectInput struct {
	Regex       string `json:"regex" validate:"required,min=1,max=512"`
	Replacement string `json:"replacement" validate:"required,min=1,max=512"`
	Permanent   bool   `json:"permanent"`
}

// CreateApplicationRedirectRequest mirrors the PHP-site redirect
// shape (components/site/CreateRedirect.vue): a path-based From, a
// path-or-URL To, and an HTTP status code. Compiled to a Traefik
// RedirectRegex middleware at deploy time.
//
// `*` in From is a wildcard segment; the matched portion is
// available as `{path}` in To. Type is one of 301 / 302 / 307 / 308.
type CreateApplicationRedirectRequest struct {
	From string `json:"from" validate:"required,min=1,max=512"`
	To   string `json:"to" validate:"required,min=1,max=512"`
	Type int    `json:"type" validate:"required,oneof=301 302 307 308"`
}

// UpdateApplicationRedirectRequest — every field optional so the
// PATCH only touches what the caller sends. Empty strings are not
// allowed for From/To (validator catches min=1); type stays in the
// 301/302/307/308 set when present.
type UpdateApplicationRedirectRequest struct {
	From *string `json:"from,omitempty" validate:"omitempty,min=1,max=512"`
	To   *string `json:"to,omitempty" validate:"omitempty,min=1,max=512"`
	Type *int    `json:"type,omitempty" validate:"omitempty,oneof=301 302 307 308"`
}

// SecurityInput holds basic-auth credentials. Empty username AND
// empty password means "clear the security middleware". We persist
// the password in build_config encrypted-at-rest via dbtype.JSONMap
// — same surface env vars use; rotation is a write, not a read.
type SecurityInput struct {
	Username string `json:"username" validate:"omitempty,max=128"`
	Password string `json:"password" validate:"omitempty,max=256"`
}

// CreateDomainRequest attaches a hostname to an application. Field
// set mirrors dokploy's domain dialog. Host is validated as a DNS-
// compatible string in the service (we don't trust validator alone
// — it doesn't catch trailing dots or leading hyphens).
//
// HTTPS defaults to true at the model level. Path is optional; when
// non-empty Traefik routes only requests under that prefix to this
// app. CreateDNSRecord + ConnectedDomainID tie into the dns module
// the same way Site creation does (see site/dto/requests.go).
type CreateDomainRequest struct {
	Host string  `json:"host" validate:"required,min=1,max=255"`
	Path *string `json:"path,omitempty" validate:"omitempty,max=255"`
	// InternalPath defaults to "/" — pass empty / omit for the
	// common case where the app's internal URLs match the external
	// path.
	InternalPath *string `json:"internal_path,omitempty" validate:"omitempty,max=255"`
	// StripPath removes the external `path` prefix before forwarding
	// to the container. Off by default — only flip when the app
	// serves at "/" internally but you want to mount it under a
	// sub-path externally.
	StripPath *bool `json:"strip_path,omitempty"`
	// ContainerPort overrides the application's internal_port for
	// this specific domain. Nil falls back to app.internal_port.
	ContainerPort *int  `json:"container_port,omitempty" validate:"omitempty,min=1,max=65535"`
	HTTPS         *bool `json:"https,omitempty"`
	// CertificateProvider is "letsencrypt" today; field exists so a
	// future migration that adds ZeroSSL doesn't need a schema bump
	// on the request body.
	CertificateProvider *string `json:"certificate_provider,omitempty" validate:"omitempty,oneof=letsencrypt"`
	// CreateDNSRecord — when true (and the domain matches a
	// connected DNS provider), the service also creates an A record
	// pointing at the docker server's public IP. Same flow Site
	// creation uses.
	CreateDNSRecord   bool    `json:"create_dns_record"`
	ConnectedDomainID *string `json:"connected_domain_id,omitempty" validate:"omitempty,ulid"`
}

// UpdateDomainRequest allows toggling per-domain config without
// re-adding the domain. Host is immutable — renaming would orphan
// the cert, so users must remove + add to change the hostname.
type UpdateDomainRequest struct {
	Path                *string `json:"path,omitempty" validate:"omitempty,max=255"`
	InternalPath        *string `json:"internal_path,omitempty" validate:"omitempty,max=255"`
	StripPath           *bool   `json:"strip_path,omitempty"`
	ContainerPort       *int    `json:"container_port,omitempty" validate:"omitempty,min=1,max=65535"`
	HTTPS               *bool   `json:"https,omitempty"`
	CertificateProvider *string `json:"certificate_provider,omitempty" validate:"omitempty,oneof=letsencrypt"`
}
