package dto

// CreateScriptRequest represents a request to create a script
type CreateScriptRequest struct {
	Name    string  `json:"name" validate:"required,max=255"`
	User    string  `json:"user" validate:"required,max=255"`
	Content string  `json:"content" validate:"required"`
	TeamID  *string `json:"team_id,omitempty"`
}

// UpdateScriptRequest represents a request to update a script
type UpdateScriptRequest struct {
	Name    *string `json:"name,omitempty" validate:"omitempty,max=255"`
	User    *string `json:"user,omitempty" validate:"omitempty,max=255"`
	Content *string `json:"content,omitempty"`
	TeamID  *string `json:"team_id,omitempty"`
}

// ExecuteScriptRequest represents a request to execute a script
type ExecuteScriptRequest struct {
	ServerIDs []string `json:"server_ids" validate:"required,min=1"`
	User      *string  `json:"user,omitempty"`
}
