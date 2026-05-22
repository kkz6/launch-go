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
