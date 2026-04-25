// Package types provides authentication-related type definitions including
// team roles and other auth domain enums.
package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// TeamRole
// =============================================================================

// TeamRole represents valid team roles
type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleEditor TeamRole = "editor"
	TeamRoleMember TeamRole = "member"
)

var allTeamRoles = []TeamRole{TeamRoleOwner, TeamRoleAdmin, TeamRoleEditor, TeamRoleMember}

// AllTeamRoles returns all valid team roles
func AllTeamRoles() []TeamRole {
	return allTeamRoles
}

// String returns the string representation of the role
func (r TeamRole) String() string {
	return string(r)
}

// Label returns the human-readable label for the role
func (r TeamRole) Label() string {
	switch r {
	case TeamRoleOwner:
		return "Owner"
	case TeamRoleAdmin:
		return "Admin"
	case TeamRoleEditor:
		return "Editor"
	case TeamRoleMember:
		return "Member"
	default:
		return string(r)
	}
}

// IsValid checks if the role is valid
func (r TeamRole) IsValid() bool {
	return enumtypes.IsValid(r, allTeamRoles...)
}

// CanManageTeam checks if this role can manage team settings
func (r TeamRole) CanManageTeam() bool {
	return r == TeamRoleOwner
}

// CanManageMembers checks if this role can manage team members
func (r TeamRole) CanManageMembers() bool {
	return r == TeamRoleOwner || r == TeamRoleAdmin
}

// CanInviteMembers checks if this role can invite new members
func (r TeamRole) CanInviteMembers() bool {
	return r == TeamRoleOwner || r == TeamRoleAdmin
}

// CanEditResources checks if this role can edit resources
func (r TeamRole) CanEditResources() bool {
	return r == TeamRoleOwner || r == TeamRoleAdmin || r == TeamRoleEditor
}

// Scan implements sql.Scanner for database reads
func (r *TeamRole) Scan(value any) error {
	return enumtypes.ScanString(r, value)
}

// Value implements driver.Valuer for database writes
func (r TeamRole) Value() (driver.Value, error) {
	return enumtypes.ValueString(r)
}

// ParseTeamRole parses a string into a TeamRole
func ParseTeamRole(s string) (TeamRole, error) {
	return enumtypes.ParseEnum(s, allTeamRoles)
}
