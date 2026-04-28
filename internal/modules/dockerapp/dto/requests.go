package dto

import "github.com/kkz6/launch-go/internal/modules/dockerapp/types"

// CreateAppRequest is the body of POST /servers/:serverId/apps.
type CreateAppRequest struct {
	Name                 string              `json:"name" validate:"required,min=1,max=64"`
	Source               types.Source        `json:"source" validate:"omitempty"`
	Image                string              `json:"image" validate:"omitempty,max=255"`
	Tag                  string              `json:"tag" validate:"omitempty,max=255"`
	RegistryCredentialID *string             `json:"registry_credential_id" validate:"omitempty,len=26"`
	RestartPolicy        types.RestartPolicy `json:"restart_policy" validate:"omitempty"`

	// Compose-source fields. Required when Source == "compose".
	ComposeYAML *string `json:"compose_yaml" validate:"omitempty"`
	ComposeEnv  *string `json:"compose_env" validate:"omitempty"`

	// Git-source fields. Required when Source == "git".
	GitRepoURL    *string `json:"git_repo_url" validate:"omitempty,url,max=512"`
	GitBranch     *string `json:"git_branch" validate:"omitempty,max=255"`
	GitDockerfile *string `json:"git_dockerfile" validate:"omitempty,max=255"`
	GitContext    *string `json:"git_context" validate:"omitempty,max=255"`
	GitToken      *string `json:"git_token" validate:"omitempty"`
}

// UpdateAppRequest is the body of PUT /servers/:serverId/apps/:id.
type UpdateAppRequest struct {
	Image                *string              `json:"image" validate:"omitempty,max=255"`
	Tag                  *string              `json:"tag" validate:"omitempty,max=255"`
	RegistryCredentialID *string              `json:"registry_credential_id" validate:"omitempty"`
	RestartPolicy        *types.RestartPolicy `json:"restart_policy" validate:"omitempty"`

	// Compose-source fields.
	ComposeYAML *string `json:"compose_yaml" validate:"omitempty"`
	ComposeEnv  *string `json:"compose_env" validate:"omitempty"`

	// Git-source fields.
	GitRepoURL    *string `json:"git_repo_url" validate:"omitempty,url,max=512"`
	GitBranch     *string `json:"git_branch" validate:"omitempty,max=255"`
	GitDockerfile *string `json:"git_dockerfile" validate:"omitempty,max=255"`
	GitContext    *string `json:"git_context" validate:"omitempty,max=255"`
	GitToken      *string `json:"git_token" validate:"omitempty"`

	// Polish — health checks, resource limits, auto-redeploy schedule.
	HealthCmd        *string `json:"health_cmd" validate:"omitempty,max=255"`
	HealthInterval   *int    `json:"health_interval_seconds" validate:"omitempty,min=1,max=86400"`
	HealthTimeout    *int    `json:"health_timeout_seconds" validate:"omitempty,min=1,max=86400"`
	HealthRetries    *int    `json:"health_retries" validate:"omitempty,min=1,max=100"`
	MemoryLimit      *string `json:"memory_limit" validate:"omitempty,max=32"`
	CPULimit         *string `json:"cpu_limit" validate:"omitempty,max=32"`
	AutoRedeployCron *string `json:"auto_redeploy_cron" validate:"omitempty,max=64"`
}

// UninstallAppRequest is the body of DELETE.
type UninstallAppRequest struct {
	RemoveData bool `json:"remove_data"`
}

// ----- Sub-resources -----

// CreateEnvVarRequest creates an env var on an app.
type CreateEnvVarRequest struct {
	Key    string `json:"key" validate:"required,min=1,max=255"`
	Value  string `json:"value" validate:"required,max=4096"`
	Secret bool   `json:"secret"`
}

// UpdateEnvVarRequest updates an env var. Empty value keeps the stored one.
type UpdateEnvVarRequest struct {
	Value  string `json:"value" validate:"omitempty,max=4096"`
	Secret *bool  `json:"secret"`
}

// CreatePortRequest publishes a host:container port.
type CreatePortRequest struct {
	HostPort      int    `json:"host_port" validate:"required,min=1,max=65535"`
	ContainerPort int    `json:"container_port" validate:"required,min=1,max=65535"`
	Protocol      string `json:"protocol" validate:"omitempty,oneof=tcp udp"`
}

// CreateVolumeRequest mounts a named volume.
type CreateVolumeRequest struct {
	Name      string `json:"name" validate:"required,min=1,max=64"`
	MountPath string `json:"mount_path" validate:"required,min=1,max=255"`
}

// CreateDomainRequest registers a domain for the app.
type CreateDomainRequest struct {
	Domain        string `json:"domain" validate:"required,min=1,max=255"`
	ContainerPort int    `json:"container_port" validate:"required,min=1,max=65535"`
	TLS           bool   `json:"tls"`
}

// UpdateDomainRequest patches a domain row.
type UpdateDomainRequest struct {
	ContainerPort *int  `json:"container_port" validate:"omitempty,min=1,max=65535"`
	TLS           *bool `json:"tls"`
}
