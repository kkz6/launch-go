// Package types contains all type definitions for the script module
package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// RunAsUser
// =============================================================================

// RunAsUser represents the user type to run a script as
type RunAsUser string

const (
	// RunAsUserRoot runs the script as the root user
	RunAsUserRoot RunAsUser = "root"

	// RunAsUserLocal runs the script as the local system user (e.g., "launch")
	RunAsUserLocal RunAsUser = "local"
)

var allRunAsUsers = []RunAsUser{
	RunAsUserRoot,
	RunAsUserLocal,
}

// AllRunAsUsers returns all valid RunAsUser values
func AllRunAsUsers() []RunAsUser {
	return allRunAsUsers
}

// String returns the string representation
func (r RunAsUser) String() string {
	return string(r)
}

// Label returns a human-readable label
func (r RunAsUser) Label() string {
	switch r {
	case RunAsUserRoot:
		return "Root"
	case RunAsUserLocal:
		return "Local"
	default:
		return string(r)
	}
}

// IsValid checks if the RunAsUser value is valid
func (r RunAsUser) IsValid() bool {
	switch r {
	case RunAsUserRoot, RunAsUserLocal:
		return true
	}

	return false
}

// Value implements the driver.Valuer interface
func (r RunAsUser) Value() (driver.Value, error) {
	return enumtypes.Value(r)
}

// Scan implements the sql.Scanner interface
func (r *RunAsUser) Scan(value any) error {
	return enumtypes.Scan(r, value)
}
