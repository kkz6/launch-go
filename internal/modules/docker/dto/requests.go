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
	Image *ImageSourceInput `json:"image,omitempty" validate:"omitempty,dive"`

	// git source.
	Git *GitSourceInput `json:"git,omitempty" validate:"omitempty,dive"`

	// dockerfile source (user-pasted Dockerfile content).
	Dockerfile *DockerfileSourceInput `json:"dockerfile,omitempty" validate:"omitempty,dive"`
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
	Git *ComposeGitInput `json:"git,omitempty" validate:"omitempty,dive"`

	// RawYAML is present when compose_source_type=raw_yaml.
	RawYAML *ComposeRawYAMLInput `json:"raw_yaml,omitempty" validate:"omitempty,dive"`
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

// CreateVolumeRequest attaches a volume to an application. Type is
// the discriminator; host_path is required for type=bind.
type CreateVolumeRequest struct {
	Name      string  `json:"name" validate:"required,min=1,max=255"`
	MountPath string  `json:"mount_path" validate:"required,min=1,max=512"`
	Type      string  `json:"type" validate:"required,oneof=named bind"`
	HostPath  *string `json:"host_path,omitempty" validate:"omitempty,max=512"`
}

// UpdateVolumeRequest is the partial-update body. Mount path can be
// changed but the volume "name" identifies the named-volume in docker
// — renaming would orphan the old one.
type UpdateVolumeRequest struct {
	MountPath *string `json:"mount_path,omitempty" validate:"omitempty,min=1,max=512"`
	HostPath  *string `json:"host_path,omitempty" validate:"omitempty,max=512"`
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

// CreateScheduleRequest adds a cron-style task that runs a command
// inside the application's container.
type CreateScheduleRequest struct {
	Cron    string `json:"cron" validate:"required,min=1,max=255"`
	Command string `json:"command" validate:"required,min=1"`
}

// UpdateScheduleRequest is the partial-update body.
type UpdateScheduleRequest struct {
	Cron    *string `json:"cron,omitempty" validate:"omitempty,min=1,max=255"`
	Command *string `json:"command,omitempty" validate:"omitempty,min=1"`
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
	// RestartPolicy: one of no / on-failure / always / unless-stopped.
	RestartPolicy *string `json:"restart_policy,omitempty" validate:"omitempty,oneof=no on-failure always unless-stopped"`
	// HealthcheckCommand is a single command run inside the container
	// for HEALTHCHECK. Empty clears the healthcheck.
	HealthcheckCommand *string `json:"healthcheck_command,omitempty" validate:"omitempty,max=512"`
	// ExtraPorts are host:container port mappings ("8080:80"). Each
	// entry is passed straight through to `docker run -p`.
	ExtraPorts []string `json:"extra_ports,omitempty" validate:"omitempty,dive,max=32"`
}

// CreateDomainRequest attaches a hostname to an application. Host is
// validated as a DNS-compatible string in the service (we don't trust
// validator alone — it doesn't catch trailing dots or leading hyphens).
//
// HTTPS defaults to true at the model level (see ApplicationDomain).
// Path is optional; when non-empty Traefik routes only requests under
// that prefix to this app.
type CreateDomainRequest struct {
	Host  string  `json:"host" validate:"required,min=1,max=255"`
	Path  *string `json:"path,omitempty" validate:"omitempty,max=255"`
	HTTPS *bool   `json:"https,omitempty"`
}

// UpdateDomainRequest allows toggling HTTPS without re-adding the
// domain. Host is immutable — renaming would orphan the cert, so users
// must remove + add to change the hostname.
type UpdateDomainRequest struct {
	HTTPS *bool   `json:"https,omitempty"`
	Path  *string `json:"path,omitempty" validate:"omitempty,max=255"`
}
