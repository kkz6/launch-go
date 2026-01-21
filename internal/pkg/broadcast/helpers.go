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

// -------------------------------------------------------------------------
// Typed Payload Structs for Complex Events
// -------------------------------------------------------------------------

// ResourceMetrics represents memory or disk metrics
type ResourceMetrics struct {
	Total float64 `json:"total"`
	Used  float64 `json:"used"`
	Free  float64 `json:"free"`
}

// ServerMetricsPayload is the typed payload for server.metrics events.
// Use this instead of inline map construction for type safety.
type ServerMetricsPayload struct {
	ServerID string          `json:"server_id"`
	Load     float64         `json:"load"`
	Memory   ResourceMetrics `json:"memory"`
	Disk     ResourceMetrics `json:"disk"`
}

// ToMap converts the payload to a map for broadcasting
func (p ServerMetricsPayload) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"server_id": p.ServerID,
		"load":      p.Load,
		"memory": map[string]interface{}{
			"total": p.Memory.Total,
			"used":  p.Memory.Used,
			"free":  p.Memory.Free,
		},
		"disk": map[string]interface{}{
			"total": p.Disk.Total,
			"used":  p.Disk.Used,
			"free":  p.Disk.Free,
		},
	}
}

// DeploymentProgressPayload is the typed payload for deployment.progress events.
type DeploymentProgressPayload struct {
	DeploymentID string `json:"deployment_id"`
	SiteID       string `json:"site_id"`
	ServerID     string `json:"server_id"`
	Step         string `json:"step"`
	Progress     int    `json:"progress"`
	Message      string `json:"message,omitempty"`
}

// ToMap converts the payload to a map for broadcasting
func (p DeploymentProgressPayload) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"deployment_id": p.DeploymentID,
		"site_id":       p.SiteID,
		"server_id":     p.ServerID,
		"step":          p.Step,
		"progress":      p.Progress,
	}
	if p.Message != "" {
		result["message"] = p.Message
	}
	return result
}
