package enums

import (
	"database/sql/driver"
	"fmt"
)

// ServerStatus represents the current state of a server
type ServerStatus string

const (
	ServerStatusNew          ServerStatus = "new"
	ServerStatusStarting     ServerStatus = "starting"
	ServerStatusProvisioning ServerStatus = "provisioning"
	ServerStatusRunning      ServerStatus = "running"
	ServerStatusPaused       ServerStatus = "paused"
	ServerStatusStopped      ServerStatus = "stopped"
	ServerStatusDeleting     ServerStatus = "deleting"
	ServerStatusArchived     ServerStatus = "archived"
	ServerStatusUnknown      ServerStatus = "unknown"
	ServerStatusFailed       ServerStatus = "failed"
)

func (s ServerStatus) String() string {
	return string(s)
}

func (s ServerStatus) Label() string {
	labels := map[ServerStatus]string{
		ServerStatusNew:          "Connecting",
		ServerStatusStarting:     "Starting",
		ServerStatusProvisioning: "Provisioning",
		ServerStatusRunning:      "Running",
		ServerStatusPaused:       "Paused",
		ServerStatusStopped:      "Stopped",
		ServerStatusDeleting:     "Deleting",
		ServerStatusArchived:     "Archived",
		ServerStatusUnknown:      "Unknown",
		ServerStatusFailed:       "Failed",
	}
	if label, ok := labels[s]; ok {
		return label
	}

	return "Unknown"
}

func (s ServerStatus) IsValid() bool {
	switch s {
	case ServerStatusNew, ServerStatusStarting, ServerStatusProvisioning,
		ServerStatusRunning, ServerStatusPaused, ServerStatusStopped,
		ServerStatusDeleting, ServerStatusArchived, ServerStatusUnknown, ServerStatusFailed:
		return true
	}

	return false
}

func (s ServerStatus) IsActive() bool {
	return s == ServerStatusRunning || s == ServerStatusProvisioning || s == ServerStatusStarting
}

func (s ServerStatus) IsTerminal() bool {
	return s == ServerStatusFailed || s == ServerStatusArchived || s == ServerStatusDeleting
}

func (s *ServerStatus) Scan(value interface{}) error {
	if value == nil {
		*s = ServerStatusUnknown
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*s = ServerStatus(v)
	case string:
		*s = ServerStatus(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServerStatus", value)
	}

	return nil
}

func (s ServerStatus) Value() (driver.Value, error) {
	return string(s), nil
}

func ParseServerStatus(s string) (ServerStatus, error) {
	status := ServerStatus(s)
	if !status.IsValid() {
		return ServerStatusUnknown, fmt.Errorf("invalid server status: %s", s)
	}

	return status, nil
}

func AllServerStatuses() []ServerStatus {
	return []ServerStatus{
		ServerStatusNew, ServerStatusStarting, ServerStatusProvisioning,
		ServerStatusRunning, ServerStatusPaused, ServerStatusStopped,
		ServerStatusDeleting, ServerStatusArchived, ServerStatusUnknown, ServerStatusFailed,
	}
}
