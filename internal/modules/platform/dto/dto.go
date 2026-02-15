package dto

import (
	"github.com/kkz6/launch-go/internal/modules/platform/models"
	"github.com/kkz6/launch-go/internal/modules/platform/repositories"
	"github.com/kkz6/launch-go/internal/modules/platform/types"
)

// PlatformUpdateResponse represents a platform update with status summary
type PlatformUpdateResponse struct {
	ID           string               `json:"id"`
	Key          string               `json:"key"`
	Title        string               `json:"title"`
	Description  string               `json:"description"`
	Severity     types.UpdateSeverity `json:"severity"`
	ServerTypes  []string             `json:"server_types,omitempty"`
	StatusCounts map[string]int64     `json:"status_counts,omitempty"`
	CreatedAt    string               `json:"created_at,omitempty"`
}

// ToPlatformUpdateResponse converts a model to response DTO
func ToPlatformUpdateResponse(update *models.PlatformUpdate) PlatformUpdateResponse {
	resp := PlatformUpdateResponse{
		ID:          update.ID,
		Key:         update.Key,
		Title:       update.Title,
		Description: update.Description,
		Severity:    update.Severity,
		ServerTypes: update.ServerTypes,
	}

	if update.CreatedAt != nil {
		resp.CreatedAt = update.CreatedAt.Format("2006-01-02T15:04:05Z")
	}

	return resp
}

// ToPlatformUpdateListResponse converts a slice of models to response DTOs
func ToPlatformUpdateListResponse(updates []models.PlatformUpdate) []PlatformUpdateResponse {
	result := make([]PlatformUpdateResponse, len(updates))
	for i, u := range updates {
		result[i] = ToPlatformUpdateResponse(&u)
	}

	return result
}

// PlatformUpdateDetailResponse includes server statuses
type PlatformUpdateDetailResponse struct {
	PlatformUpdateResponse
	ServerStatuses []ServerUpdateStatusResponse `json:"server_statuses"`
}

// ServerUpdateStatusResponse represents a server's status for an update
type ServerUpdateStatusResponse struct {
	ID           string                   `json:"id"`
	ServerID     string                   `json:"server_id"`
	ServerName   string                   `json:"server_name"`
	Status       types.ServerUpdateStatus `json:"status"`
	TaskID       *string                  `json:"task_id,omitempty"`
	ErrorMessage *string                  `json:"error_message,omitempty"`
	CompletedAt  *string                  `json:"completed_at,omitempty"`
}

// ToServerUpdateStatusResponse converts a row to response DTO
func ToServerUpdateStatusResponse(row repositories.ServerStatusRow) ServerUpdateStatusResponse {
	return ServerUpdateStatusResponse{
		ID:           row.ID,
		ServerID:     row.ServerID,
		ServerName:   row.ServerName,
		Status:       row.Status,
		TaskID:       row.TaskID,
		ErrorMessage: row.ErrorMessage,
		CompletedAt:  row.CompletedAt,
	}
}

// RunUpdateRequest is the request body for running an update on a server
type RunUpdateRequest struct {
	ServerID string `json:"server_id" validate:"required"`
}
