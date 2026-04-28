package dto

import (
	"github.com/kkz6/launch-go/internal/modules/dockerapp/models"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// AppResponse is the JSON shape of a docker application.
type AppResponse struct {
	ID                   string  `json:"id"`
	ServerID             string  `json:"server_id"`
	Name                 string  `json:"name"`
	Source               string  `json:"source"`
	Image                string  `json:"image"`
	Tag                  string  `json:"tag"`
	ImageRef             string  `json:"image_ref"`
	Container            string  `json:"container"`
	RegistryCredentialID *string `json:"registry_credential_id,omitempty"`
	RestartPolicy        string  `json:"restart_policy"`
	Status               string  `json:"status"`
	StatusLabel          string  `json:"status_label"`
	LastError            *string `json:"last_error,omitempty"`
	TaskID               *string `json:"task_id,omitempty"`
	DeployedAt           *string `json:"deployed_at,omitempty"`
	CreatedAt            *string `json:"created_at,omitempty"`
	UpdatedAt            *string `json:"updated_at,omitempty"`

	// Compose source — YAML is returned, .env content is not (sensitive).
	ComposeYAML *string `json:"compose_yaml,omitempty"`
	// HasComposeEnv indicates whether a .env was set, without exposing its content.
	HasComposeEnv  bool   `json:"has_compose_env"`
	ComposeProject string `json:"compose_project,omitempty"`

	// Git source — URL/branch/etc. are returned, the access token is not.
	GitRepoURL    *string `json:"git_repo_url,omitempty"`
	GitBranch     *string `json:"git_branch,omitempty"`
	GitDockerfile *string `json:"git_dockerfile,omitempty"`
	GitContext    *string `json:"git_context,omitempty"`
	GitCommitSHA  *string `json:"git_commit_sha,omitempty"`
	HasGitToken   bool    `json:"has_git_token"`

	EnvVars []EnvVarResponse `json:"env_vars,omitempty"`
	Ports   []PortResponse   `json:"ports,omitempty"`
	Volumes []VolumeResponse `json:"volumes,omitempty"`
	Domains []DomainResponse `json:"domains,omitempty"`
}

// EnvVarResponse is the shape of an env-var row. The decrypted value is
// only included when secret=false.
type EnvVarResponse struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Secret bool   `json:"secret"`
}

// PortResponse is the shape of a port row.
type PortResponse struct {
	ID            string `json:"id"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// VolumeResponse is the shape of a volume row.
type VolumeResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	HostName  string `json:"host_name"`
	MountPath string `json:"mount_path"`
}

// DomainResponse is the shape of a domain row.
type DomainResponse struct {
	ID            string `json:"id"`
	Domain        string `json:"domain"`
	ContainerPort int    `json:"container_port"`
	TLS           bool   `json:"tls"`
}

// ToAppResponse converts a model. Sub-resources are included only when
// they are populated on the model (via preload).
func ToAppResponse(a *models.App) AppResponse {
	resp := AppResponse{
		ID:                   a.ID,
		ServerID:             a.ServerID,
		Name:                 a.Name,
		Source:               a.Source.String(),
		Image:                a.Image,
		Tag:                  a.Tag,
		ImageRef:             a.ImageRef(),
		Container:            a.Container(),
		RegistryCredentialID: a.RegistryCredentialID,
		RestartPolicy:        a.RestartPolicy.String(),
		Status:               a.Status.String(),
		StatusLabel:          a.Status.Label(),
		LastError:            a.LastError,
		TaskID:               a.TaskID,
		DeployedAt:           pkgdto.FormatTime(a.DeployedAt),
		CreatedAt:            pkgdto.FormatTime(a.CreatedAt),
		UpdatedAt:            pkgdto.FormatTime(a.UpdatedAt),
		HasComposeEnv:        a.ComposeEnv != nil,
		HasGitToken:          a.GitToken != nil,
	}
	if a.Source.String() == "compose" {
		resp.ComposeProject = a.ComposeProject()
		if a.ComposeYAML != nil {
			resp.ComposeYAML = a.ComposeYAML
		}
	}
	if a.Source.String() == "git" {
		resp.GitRepoURL = a.GitRepoURL
		resp.GitBranch = a.GitBranch
		resp.GitDockerfile = a.GitDockerfile
		resp.GitContext = a.GitContext
		resp.GitCommitSHA = a.GitCommitSHA
	}
	for _, e := range a.EnvVars {
		out := EnvVarResponse{ID: e.ID, Key: e.Key, Secret: e.Secret}
		if !e.Secret {
			out.Value = e.Value.String()
		}
		resp.EnvVars = append(resp.EnvVars, out)
	}
	for _, p := range a.Ports {
		resp.Ports = append(resp.Ports, PortResponse{
			ID: p.ID, HostPort: p.HostPort, ContainerPort: p.ContainerPort, Protocol: p.Protocol,
		})
	}
	for _, v := range a.Volumes {
		resp.Volumes = append(resp.Volumes, ToVolumeResponse(a, &v))
	}
	for _, d := range a.Domains {
		resp.Domains = append(resp.Domains, DomainResponse{
			ID: d.ID, Domain: d.Domain, ContainerPort: d.ContainerPort, TLS: d.TLS,
		})
	}
	return resp
}

// ToAppResponseList converts a slice of apps.
func ToAppResponseList(in []models.App) []AppResponse {
	out := make([]AppResponse, len(in))
	for i := range in {
		out[i] = ToAppResponse(&in[i])
	}
	return out
}

// ToVolumeResponse projects a volume row, deriving the host volume name
// from the app + the user-supplied suffix.
func ToVolumeResponse(a *models.App, v *models.Volume) VolumeResponse {
	return VolumeResponse{
		ID:        v.ID,
		Name:      v.Name,
		HostName:  "launch-app-" + a.Name + "-" + v.Name,
		MountPath: v.MountPath,
	}
}
