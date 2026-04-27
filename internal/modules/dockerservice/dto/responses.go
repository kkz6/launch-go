package dto

import (
	"github.com/kkz6/launch-go/internal/modules/dockerservice/models"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/types"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// DockerServiceResponse is the JSON shape of a docker service.
type DockerServiceResponse struct {
	ID           string  `json:"id"`
	ServerID     string  `json:"server_id"`
	Kind         string  `json:"kind"`
	KindLabel    string  `json:"kind_label"`
	Status       string  `json:"status"`
	StatusLabel  string  `json:"status_label"`
	Image        string  `json:"image"`
	Container    string  `json:"container"`
	Volume       string  `json:"volume"`
	DatabaseName *string `json:"database_name,omitempty"`
	Username     *string `json:"username,omitempty"`
	LastError    *string `json:"last_error,omitempty"`
	TaskID       *string `json:"task_id,omitempty"`
	CreatedAt    *string `json:"created_at,omitempty"`
	UpdatedAt    *string `json:"updated_at,omitempty"`
}

// DockerServiceLogsResponse is returned by GET /logs.
type DockerServiceLogsResponse struct {
	Kind    types.Kind `json:"kind"`
	Tail    int        `json:"tail"`
	Output  string     `json:"output"`
	Success bool       `json:"success"`
}

// ToDockerServiceResponse converts a model to its API response shape.
func ToDockerServiceResponse(m *models.DockerService) DockerServiceResponse {
	return DockerServiceResponse{
		ID:           m.ID,
		ServerID:     m.ServerID,
		Kind:         m.Kind.String(),
		KindLabel:    m.Kind.Label(),
		Status:       m.Status.String(),
		StatusLabel:  m.Status.Label(),
		Image:        m.Image,
		Container:    m.Container,
		Volume:       m.Volume,
		DatabaseName: m.DatabaseName,
		Username:     m.Username,
		LastError:    m.LastError,
		TaskID:       m.TaskID,
		CreatedAt:    pkgdto.FormatTime(m.CreatedAt),
		UpdatedAt:    pkgdto.FormatTime(m.UpdatedAt),
	}
}

// ToDockerServiceResponseList converts a slice of models.
func ToDockerServiceResponseList(in []models.DockerService) []DockerServiceResponse {
	out := make([]DockerServiceResponse, len(in))
	for i := range in {
		out[i] = ToDockerServiceResponse(&in[i])
	}
	return out
}
