package broadcast

import (
	"encoding/json"
	"strings"
)

const (
	AdminResourcesChannel = "admin.resources"
	AdminResourceChanged  = "admin.resource.changed"
)

// AdminResourceChange is an invalidation hint, never a cross-team model payload.
// Staff clients refetch their authorized REST views after receiving it.
type AdminResourceChange struct {
	Resource  string `json:"resource"`
	Action    string `json:"action"`
	TeamID    string `json:"team_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	ServerID  string `json:"server_id,omitempty"`
	SiteID    string `json:"site_id,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

// AdminChangeForTeamEvent projects lifecycle events onto routing IDs and
// allowlisted failure codes. Logs, metrics and progress do not invalidate lists.
func AdminChangeForTeamEvent(teamID, event string, data any) *AdminResourceChange {
	resource, action, ok := strings.Cut(event, ".")
	if !ok || action == "" || teamID == "" {
		return nil
	}
	switch resource {
	case "server", "site", "docker", "user", "team", "subscription", "task", "deployment":
	default:
		return nil
	}
	last := action[strings.LastIndex(action, ".")+1:]
	if strings.HasSuffix(last, "_progress") || strings.HasSuffix(last, "_output") {
		return nil
	}
	if resource == "task" && (last == "step_started" || last == "step_completed" || last == "status" || last == "error" || last == "software_installed") {
		return nil
	}
	switch last {
	case "output", "progress", "metrics", "logs", "log", "gha_steps":
		return nil
	}
	change := &AdminResourceChange{Resource: resource, Action: action, TeamID: teamID}
	// Sources use either response structs or maps. Decoding only these fields
	// prevents credentials, task output and nested model data from escaping.
	var ids struct {
		ID        string `json:"id"`
		UserID    string `json:"user_id"`
		ServerID  string `json:"server_id"`
		SiteID    string `json:"site_id"`
		ErrorCode string `json:"error_code"`
	}
	if fields, ok := data.(map[string]any); ok {
		ids.ID, _ = fields["id"].(string)
		ids.UserID, _ = fields["user_id"].(string)
		ids.ServerID, _ = fields["server_id"].(string)
		ids.SiteID, _ = fields["site_id"].(string)
		ids.ErrorCode, _ = fields["error_code"].(string)
	} else if encoded, err := json.Marshal(data); err == nil {
		_ = json.Unmarshal(encoded, &ids)
	}
	change.UserID, change.ServerID, change.SiteID = ids.UserID, ids.ServerID, ids.SiteID
	if event == "server.deletion_failed" {
		switch ids.ErrorCode {
		case "provider_unauthorized", "provider_forbidden":
			change.ErrorCode = ids.ErrorCode
		}
	}
	switch resource {
	case "server":
		if change.ServerID == "" {
			change.ServerID = ids.ID
		}
	case "site":
		if change.SiteID == "" {
			change.SiteID = ids.ID
		}
	case "user":
		if change.UserID == "" {
			change.UserID = ids.ID
		}
	}
	return change
}
