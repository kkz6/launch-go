package enums

import (
	"database/sql/driver"
	"fmt"
)

// ServiceStatus represents the status of a service
type ServiceStatus string

const (
	ServiceStatusPending    ServiceStatus = "pending"
	ServiceStatusInstalling ServiceStatus = "installing"
	ServiceStatusFailed     ServiceStatus = "failed"
	ServiceStatusInstalled  ServiceStatus = "installed"
	ServiceStatusStopped    ServiceStatus = "stopped"
	ServiceStatusRunning    ServiceStatus = "running"
)

func (s ServiceStatus) String() string {
	return string(s)
}

func (s ServiceStatus) Label() string {
	labels := map[ServiceStatus]string{
		ServiceStatusPending:    "Pending",
		ServiceStatusInstalling: "Installing",
		ServiceStatusFailed:     "Failed",
		ServiceStatusInstalled:  "Installed",
		ServiceStatusStopped:    "Stopped",
		ServiceStatusRunning:    "Running",
	}
	if label, ok := labels[s]; ok {
		return label
	}

	return "Unknown"
}

func (s ServiceStatus) IsValid() bool {
	switch s {
	case ServiceStatusPending, ServiceStatusInstalling, ServiceStatusFailed,
		ServiceStatusInstalled, ServiceStatusStopped, ServiceStatusRunning:
		return true
	}

	return false
}

func (s ServiceStatus) IsActive() bool {
	return s == ServiceStatusRunning || s == ServiceStatusInstalled
}

func (s *ServiceStatus) Scan(value interface{}) error {
	if value == nil {
		*s = ServiceStatusPending
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*s = ServiceStatus(v)
	case string:
		*s = ServiceStatus(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServiceStatus", value)
	}

	return nil
}

func (s ServiceStatus) Value() (driver.Value, error) {
	return string(s), nil
}

func ParseServiceStatus(str string) (ServiceStatus, error) {
	s := ServiceStatus(str)
	if !s.IsValid() {
		return ServiceStatusPending, fmt.Errorf("invalid service status: %s", str)
	}

	return s, nil
}

func AllServiceStatuses() []ServiceStatus {
	return []ServiceStatus{
		ServiceStatusPending, ServiceStatusInstalling, ServiceStatusFailed,
		ServiceStatusInstalled, ServiceStatusStopped, ServiceStatusRunning,
	}
}
