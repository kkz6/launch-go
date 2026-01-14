package enums

// TeamRole represents valid team roles
type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleEditor TeamRole = "editor"
	TeamRoleMember TeamRole = "member"
)

// IsValid checks if the role is valid
func (r TeamRole) IsValid() bool {
	switch r {
	case TeamRoleOwner, TeamRoleAdmin, TeamRoleEditor, TeamRoleMember:
		return true
	}

	return false
}

// String returns the string representation of the role
func (r TeamRole) String() string {
	return string(r)
}
