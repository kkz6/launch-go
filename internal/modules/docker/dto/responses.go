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
	ID             string                 `json:"id"`
	TeamID         string                 `json:"team_id"`
	ServerID       string                 `json:"server_id"`
	ProjectID      string                 `json:"project_id"`
	Name           string                 `json:"name"`
	SourceType     string                 `json:"source_type"`
	SourceConfig   map[string]any         `json:"source_config,omitempty"`
	BuildType      *string                `json:"build_type,omitempty"`
	BuildConfig    map[string]any         `json:"build_config,omitempty"`
	Status         string                 `json:"status"`
	ContainerID    *string                `json:"container_id,omitempty"`
	LastDeployedAt *time.Time             `json:"last_deployed_at,omitempty"`
	CreatedAt      *time.Time             `json:"created_at,omitempty"`
	UpdatedAt      *time.Time             `json:"updated_at,omitempty"`
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
