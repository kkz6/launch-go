package dto

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/script/models"
	scripttypes "github.com/kkz6/launch-go/internal/modules/script/types"
)

// ScriptResponse represents a script in API responses
type ScriptResponse struct {
	ID        string                `json:"id"`
	UserID    string                `json:"user_id"`
	TeamID    *string               `json:"team_id,omitempty"`
	Name      string                `json:"name"`
	RunAs     scripttypes.RunAsUser `json:"run_as"`
	Content   string                `json:"content"`
	CreatedAt *time.Time            `json:"created_at,omitempty"`
	UpdatedAt *time.Time            `json:"updated_at,omitempty"`
}

// ToScriptResponse converts a Script model to ScriptResponse
func ToScriptResponse(s *models.Script) *ScriptResponse {
	return &ScriptResponse{
		ID:        s.ID,
		UserID:    s.UserID,
		TeamID:    s.TeamID,
		Name:      s.Name,
		RunAs:     s.RunAs,
		Content:   s.Content,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

// ScriptExecutionResponse represents an execution in API responses
type ScriptExecutionResponse struct {
	ID         uint64                 `json:"id"`
	ScriptID   string                 `json:"script_id"`
	ServerID   string                 `json:"server_id"`
	ServerName string                 `json:"server_name"`
	BatchID    *string                `json:"batch_id,omitempty"`
	RunAs      *scripttypes.RunAsUser `json:"run_as,omitempty"`
	Status     string                 `json:"status"`
	ExitCode   *int                   `json:"exit_code,omitempty"`
	Output     *string                `json:"output,omitempty"`
	StartedAt  *time.Time             `json:"started_at,omitempty"`
	FinishedAt *time.Time             `json:"finished_at,omitempty"`
	CreatedAt  *time.Time             `json:"created_at,omitempty"`
}

// ToExecutionResponse converts a ScriptExecution to response
func ToExecutionResponse(e *models.ScriptExecution) *ScriptExecutionResponse {
	serverName := ""
	if e.Server != nil {
		serverName = e.Server.Name
	}

	return &ScriptExecutionResponse{
		ID:         e.ID,
		ScriptID:   e.ScriptID,
		ServerID:   e.ServerID,
		ServerName: serverName,
		BatchID:    e.BatchID,
		RunAs:      e.RunAs,
		Status:     string(e.Status),
		ExitCode:   e.ExitCode,
		Output:     e.Output,
		StartedAt:  e.StartedAt,
		FinishedAt: e.FinishedAt,
		CreatedAt:  e.CreatedAt,
	}
}

// ExecuteScriptResponse represents the response after triggering execution
type ExecuteScriptResponse struct {
	BatchID    string                     `json:"batch_id"`
	Executions []*ScriptExecutionResponse `json:"executions"`
}
