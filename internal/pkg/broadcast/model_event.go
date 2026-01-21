package broadcast

// BuildModelEventPayload creates a standardized payload for model events.
// This is used by both Hub and RedisBroadcaster to ensure consistent
// payload structure for model CRUD broadcasts.
//
// The payload structure follows the convention:
//
//	{
//	    "id":      modelID,
//	    "model":   modelName,
//	    "action":  action,
//	    "team_id": teamID,
//	    "data":    data,
//	}
//
// Example usage:
//
//	payload := broadcast.BuildModelEventPayload("server", server.ID, "created", teamID, serverData)
func BuildModelEventPayload(modelName, modelID, action, teamID string, data any) map[string]any {
	return map[string]any{
		"id":      modelID,
		"model":   modelName,
		"action":  action,
		"team_id": teamID,
		"data":    data,
	}
}

// ModelEventName returns the standard event name for a model action.
// Format: "{modelName}.{action}" (e.g., "server.created", "site.updated")
//
// Example usage:
//
//	eventName := broadcast.ModelEventName("server", "created") // returns "server.created"
func ModelEventName(modelName, action string) string {
	return modelName + "." + action
}

// ModelEventPayload is a typed struct version of the model event payload.
// Use this when you need type safety or want to marshal/unmarshal the payload.
type ModelEventPayload struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Action string `json:"action"`
	TeamID string `json:"team_id"`
	Data   any    `json:"data"`
}

// NewModelEventPayload creates a new typed model event payload.
func NewModelEventPayload(modelName, modelID, action, teamID string, data any) *ModelEventPayload {
	return &ModelEventPayload{
		ID:     modelID,
		Model:  modelName,
		Action: action,
		TeamID: teamID,
		Data:   data,
	}
}

// ToMap converts the payload to a map for broadcasting.
func (p *ModelEventPayload) ToMap() map[string]any {
	return BuildModelEventPayload(p.Model, p.ID, p.Action, p.TeamID, p.Data)
}
