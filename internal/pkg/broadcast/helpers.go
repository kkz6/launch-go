package broadcast

// Action represents the type of broadcast action
type Action string

const (
	ActionCreated Action = "created"
	ActionUpdated Action = "updated"
	ActionDeleted Action = "deleted"
)

// Broadcastable defines a minimal interface for payload generation.
// This is a subset of models.Broadcastable which adds GetTeamID().
// Use this interface for generic payload helper functions.
// For full model broadcasting with team context, see pkg/models.Broadcastable.
type Broadcastable interface {
	BroadcastName() string
	BroadcastPayload() map[string]interface{}
}

// Payload creates a standard broadcast payload for a model
func Payload[T Broadcastable](model T, action Action) map[string]interface{} {
	return map[string]interface{}{
		"action": string(action),
		"model":  model.BroadcastName(),
		"data":   model.BroadcastPayload(),
	}
}

// PayloadWithExtra creates a broadcast payload with additional fields
func PayloadWithExtra[T Broadcastable](model T, action Action, extra map[string]interface{}) map[string]interface{} {
	payload := Payload(model, action)
	for k, v := range extra {
		payload[k] = v
	}
	return payload
}

// SimplePayload creates a simple broadcast payload without model interface requirement
func SimplePayload(modelName string, action Action, data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"action": string(action),
		"model":  modelName,
		"data":   data,
	}
}

// StatusPayload creates a status update payload commonly used for async operations
func StatusPayload(modelName, id, status string) map[string]interface{} {
	return map[string]interface{}{
		"model":  modelName,
		"id":     id,
		"status": status,
	}
}

// ProgressPayload creates a progress update payload for long-running operations
func ProgressPayload(modelName, id string, progress int, message string) map[string]interface{} {
	payload := map[string]interface{}{
		"model":    modelName,
		"id":       id,
		"progress": progress,
	}
	if message != "" {
		payload["message"] = message
	}
	return payload
}

// ErrorPayload creates an error notification payload
func ErrorPayload(modelName, id, errorMsg string) map[string]interface{} {
	return map[string]interface{}{
		"model": modelName,
		"id":    id,
		"error": errorMsg,
	}
}

// EventName generates a standard event name for broadcast
// Format: "{model}.{action}" (e.g., "server.created", "site.updated")
func EventName(modelName string, action Action) string {
	return modelName + "." + string(action)
}
