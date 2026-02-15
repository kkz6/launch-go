package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// UpdateSeverity represents the severity level of a platform update
type UpdateSeverity string

const (
	SeverityInfo     UpdateSeverity = "info"
	SeverityWarning  UpdateSeverity = "warning"
	SeverityCritical UpdateSeverity = "critical"
)

var allSeverities = []UpdateSeverity{SeverityInfo, SeverityWarning, SeverityCritical}

func AllSeverities() []UpdateSeverity { return allSeverities }

func (s UpdateSeverity) String() string { return string(s) }

func (s UpdateSeverity) Label() string {
	switch s {
	case SeverityInfo:
		return "Info"
	case SeverityWarning:
		return "Warning"
	case SeverityCritical:
		return "Critical"
	default:
		return string(s)
	}
}

func (s UpdateSeverity) IsValid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	}

	return false
}

func (s *UpdateSeverity) Scan(value any) error        { return enumtypes.ScanString(s, value) }
func (s UpdateSeverity) Value() (driver.Value, error) { return enumtypes.ValueString(s) }

// ServerUpdateStatus represents the status of a platform update on a specific server
type ServerUpdateStatus string

const (
	UpdateStatusPending   ServerUpdateStatus = "pending"
	UpdateStatusRunning   ServerUpdateStatus = "running"
	UpdateStatusCompleted ServerUpdateStatus = "completed"
	UpdateStatusFailed    ServerUpdateStatus = "failed"
	UpdateStatusSkipped   ServerUpdateStatus = "skipped"
)

var allStatuses = []ServerUpdateStatus{
	UpdateStatusPending,
	UpdateStatusRunning,
	UpdateStatusCompleted,
	UpdateStatusFailed,
	UpdateStatusSkipped,
}

func AllStatuses() []ServerUpdateStatus { return allStatuses }

func (s ServerUpdateStatus) String() string { return string(s) }

func (s ServerUpdateStatus) Label() string {
	switch s {
	case UpdateStatusPending:
		return "Pending"
	case UpdateStatusRunning:
		return "Running"
	case UpdateStatusCompleted:
		return "Completed"
	case UpdateStatusFailed:
		return "Failed"
	case UpdateStatusSkipped:
		return "Skipped"
	default:
		return string(s)
	}
}

func (s ServerUpdateStatus) IsValid() bool {
	switch s {
	case UpdateStatusPending, UpdateStatusRunning, UpdateStatusCompleted, UpdateStatusFailed, UpdateStatusSkipped:
		return true
	}

	return false
}

func (s *ServerUpdateStatus) Scan(value any) error        { return enumtypes.ScanString(s, value) }
func (s ServerUpdateStatus) Value() (driver.Value, error) { return enumtypes.ValueString(s) }
