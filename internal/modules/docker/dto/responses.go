package dto

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
)

// ProjectResponse is the API representation of a docker project.
type ProjectResponse struct {
	ID                string     `json:"id"`
	TeamID            string     `json:"team_id"`
	ServerID          string     `json:"server_id"`
	Name              string     `json:"name"`
	Description       *string    `json:"description,omitempty"`
	ApplicationsCount int64      `json:"applications_count"`
	ComposesCount     int64      `json:"composes_count"`
	DatabasesCount    int64      `json:"databases_count"`
	CreatedAt         *time.Time `json:"created_at,omitempty"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
}

// ToProjectResponse maps a Project model to its API response shape.
func ToProjectResponse(p *models.Project) *ProjectResponse {
	return &ProjectResponse{
		ID:                p.ID,
		TeamID:            p.TeamID,
		ServerID:          p.ServerID,
		Name:              p.Name,
		Description:       p.Description,
		ApplicationsCount: p.ApplicationsCount,
		ComposesCount:     p.ComposesCount,
		DatabasesCount:    p.DatabasesCount,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
}

// ApplicationResponse is the API representation of a docker application.
//
// SourceConfig and BuildConfig are returned as opaque JSON-encoded blobs —
// the frontend interprets them based on SourceType. Doing this here (rather
// than projecting each variant into its own field) keeps the API stable as
// we add fields to one source type without touching the others.
type ApplicationResponse struct {
	ID             string         `json:"id"`
	TeamID         string         `json:"team_id"`
	ServerID       string         `json:"server_id"`
	ProjectID      string         `json:"project_id"`
	Name           string         `json:"name"`
	InternalPort   int            `json:"internal_port"`
	SourceType     string         `json:"source_type"`
	SourceConfig   map[string]any `json:"source_config,omitempty"`
	BuildType      *string        `json:"build_type,omitempty"`
	BuildConfig    map[string]any `json:"build_config,omitempty"`
	Status         string         `json:"status"`
	ContainerID    *string        `json:"container_id,omitempty"`
	LastDeployedAt *time.Time     `json:"last_deployed_at,omitempty"`
	CreatedAt      *time.Time     `json:"created_at,omitempty"`
	UpdatedAt      *time.Time     `json:"updated_at,omitempty"`
}

// DeploymentResponse is the API representation of a deploy attempt.
//
// Mirrors models.Deployment but skips internal fields (LogPath) we
// haven't wired the UI for yet (slice 2d).
type DeploymentResponse struct {
	ID         string     `json:"id"`
	TeamID     string     `json:"team_id"`
	ServerID   string     `json:"server_id"`
	TargetType string     `json:"target_type"`
	TargetID   string     `json:"target_id"`
	Status     string     `json:"status"`
	CommitSHA  *string    `json:"commit_sha,omitempty"`
	CommitMsg  *string    `json:"commit_msg,omitempty"`
	ImageRef   *string    `json:"image_ref,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      *string    `json:"error,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

// ComposeResponse is the API representation of a docker compose stack.
type ComposeResponse struct {
	ID                string         `json:"id"`
	TeamID            string         `json:"team_id"`
	ServerID          string         `json:"server_id"`
	ProjectID         string         `json:"project_id"`
	Name              string         `json:"name"`
	ComposeSourceType string         `json:"compose_source_type"`
	SourceConfig      map[string]any `json:"source_config,omitempty"`
	ComposeFilePath   *string        `json:"compose_file_path,omitempty"`
	// RawYAML is omitted from list responses to keep them light; pulled
	// in for single-compose Show responses where the user is editing.
	RawYAML        *string    `json:"raw_yaml,omitempty"`
	Status         string     `json:"status"`
	LastDeployedAt *time.Time `json:"last_deployed_at,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}

// ToComposeResponse converts a Compose model to the API shape. The
// includeRaw flag controls whether the raw YAML body (potentially KB-
// scale) is included; list endpoints should pass false.
func ToComposeResponse(c *models.Compose, includeRaw bool) *ComposeResponse {
	resp := &ComposeResponse{
		ID:                c.ID,
		TeamID:            c.TeamID,
		ServerID:          c.ServerID,
		ProjectID:         c.ProjectID,
		Name:              c.Name,
		ComposeSourceType: c.ComposeSourceType,
		SourceConfig:      map[string]any(c.SourceConfig),
		ComposeFilePath:   c.ComposeFilePath,
		Status:            string(c.Status),
		LastDeployedAt:    c.LastDeployedAt,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
	if includeRaw {
		resp.RawYAML = c.RawYAML
	}
	return resp
}

// DatabaseCredentials is the shape of the auto-generated DB secrets we
// reveal to the user when they explicitly ask. Mirrored from the
// services.Credentials struct.
type DatabaseCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// DatabaseResponse is the API representation of a managed database.
//
// Credentials are absent unless the caller asked to reveal them — the
// reveal flag controls whether the password is included so we don't
// leak it on list endpoints.
type DatabaseResponse struct {
	ID            string               `json:"id"`
	TeamID        string               `json:"team_id"`
	ServerID      string               `json:"server_id"`
	ProjectID     string               `json:"project_id"`
	Name          string               `json:"name"`
	Engine        string               `json:"engine"`
	EngineVersion string               `json:"engine_version"`
	ImageTag      *string              `json:"image_tag,omitempty"`
	ExternalPort  *int                 `json:"external_port,omitempty"`
	Status        string               `json:"status"`
	Credentials   *DatabaseCredentials `json:"credentials,omitempty"`
	CreatedAt     *time.Time           `json:"created_at,omitempty"`
	UpdatedAt     *time.Time           `json:"updated_at,omitempty"`
}

// ToDatabaseResponse renders a Database model. The caller passes
// reveal=true only on the explicit-reveal show endpoint; list +
// default get omit the password entirely.
func ToDatabaseResponse(d *models.Database, _ bool) *DatabaseResponse {
	return &DatabaseResponse{
		ID:            d.ID,
		TeamID:        d.TeamID,
		ServerID:      d.ServerID,
		ProjectID:     d.ProjectID,
		Name:          d.Name,
		Engine:        string(d.Engine),
		EngineVersion: d.EngineVersion,
		ImageTag:      d.ImageTag,
		ExternalPort:  d.ExternalPort,
		Status:        string(d.Status),
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

// DomainResponse is the API representation of an application domain.
type DomainResponse struct {
	ID            string     `json:"id"`
	ApplicationID string     `json:"application_id"`
	Host          string     `json:"host"`
	Path          *string    `json:"path,omitempty"`
	HTTPS         bool       `json:"https"`
	CertificateID *string    `json:"certificate_id,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

// ToDomainResponse maps a domain model to the API response shape.
func ToDomainResponse(d *models.ApplicationDomain) *DomainResponse {
	return &DomainResponse{
		ID:            d.ID,
		ApplicationID: d.ApplicationID,
		Host:          d.Host,
		Path:          d.Path,
		HTTPS:         d.HTTPS,
		CertificateID: d.CertificateID,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

// ToDeploymentResponse maps a Deployment model to the API response.
func ToDeploymentResponse(d *models.Deployment) *DeploymentResponse {
	return &DeploymentResponse{
		ID:         d.ID,
		TeamID:     d.TeamID,
		ServerID:   d.ServerID,
		TargetType: d.TargetType,
		TargetID:   d.TargetID,
		Status:     string(d.Status),
		CommitSHA:  d.CommitSHA,
		CommitMsg:  d.CommitMsg,
		ImageRef:   d.ImageRef,
		StartedAt:  d.StartedAt,
		FinishedAt: d.FinishedAt,
		Error:      d.Error,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// ToApplicationResponse maps an Application model to its API response shape.
func ToApplicationResponse(a *models.Application) *ApplicationResponse {
	var buildType *string
	if a.BuildType != nil {
		bt := string(*a.BuildType)
		buildType = &bt
	}
	return &ApplicationResponse{
		ID:             a.ID,
		TeamID:         a.TeamID,
		ServerID:       a.ServerID,
		ProjectID:      a.ProjectID,
		Name:           a.Name,
		InternalPort:   a.InternalPort,
		SourceType:     string(a.SourceType),
		SourceConfig:   map[string]any(a.SourceConfig),
		BuildType:      buildType,
		BuildConfig:    map[string]any(a.BuildConfig),
		Status:         string(a.Status),
		ContainerID:    a.ContainerID,
		LastDeployedAt: a.LastDeployedAt,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
}
