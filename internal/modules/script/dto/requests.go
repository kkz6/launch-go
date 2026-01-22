package dto

import scripttypes "github.com/kkz6/launch-go/internal/modules/script/types"

// CreateScriptRequest represents a request to create a script
type CreateScriptRequest struct {
	Name    string                `json:"name" validate:"required,max=255"`
	RunAs   scripttypes.RunAsUser `json:"run_as" validate:"required,oneof=root local"`
	Content string                `json:"content" validate:"required"`
	TeamID  *string               `json:"team_id,omitempty"`
}

// UpdateScriptRequest represents a request to update a script
type UpdateScriptRequest struct {
	Name    *string                `json:"name,omitempty" validate:"omitempty,max=255"`
	RunAs   *scripttypes.RunAsUser `json:"run_as,omitempty" validate:"omitempty,oneof=root local"`
	Content *string                `json:"content,omitempty"`
	TeamID  *string                `json:"team_id,omitempty"`
}

// ExecuteScriptRequest represents a request to execute a script
type ExecuteScriptRequest struct {
	ServerIDs []string               `json:"server_ids" validate:"required,min=1"`
	RunAs     *scripttypes.RunAsUser `json:"run_as,omitempty" validate:"omitempty,oneof=root local"`
}
