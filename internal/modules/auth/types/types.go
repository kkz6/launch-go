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

var teamRoleLabels = map[TeamRole]string{
	TeamRoleOwner:  "Owner",
	TeamRoleAdmin:  "Admin",
	TeamRoleEditor: "Editor",
	TeamRoleMember: "Member",
}

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
	return enumtypes.Label(r, teamRoleLabels, string(r))
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

// =============================================================================
// UserStatus
// =============================================================================

// UserStatus represents the account status of a user
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
)

var allUserStatuses = []UserStatus{UserStatusActive, UserStatusSuspended}

var userStatusLabels = map[UserStatus]string{
	UserStatusActive:    "Active",
	UserStatusSuspended: "Suspended",
}

// AllUserStatuses returns all valid user statuses
func AllUserStatuses() []UserStatus {
	return allUserStatuses
}

// String returns the string representation of the status
func (s UserStatus) String() string {
	return string(s)
}

// Label returns the human-readable label for the status
func (s UserStatus) Label() string {
	return enumtypes.Label(s, userStatusLabels, string(s))
}

// IsValid checks if the status is valid
func (s UserStatus) IsValid() bool {
	return enumtypes.IsValid(s, allUserStatuses...)
}

// Scan implements sql.Scanner for database reads
func (s *UserStatus) Scan(value any) error {
	return enumtypes.ScanString(s, value)
}

// Value implements driver.Valuer for database writes
func (s UserStatus) Value() (driver.Value, error) {
	return enumtypes.ValueString(s)
}

// ParseUserStatus parses a string into a UserStatus
func ParseUserStatus(s string) (UserStatus, error) {
	return enumtypes.ParseEnum(s, allUserStatuses)
}
