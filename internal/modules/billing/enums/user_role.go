package enums

// UserRole represents user roles for permission checking
type UserRole string

const (
	UserRoleCustomer UserRole = "customer"
	UserRoleManager  UserRole = "manager"
	UserRoleAdmin    UserRole = "admin"
)

// String returns the string representation of the user role
func (u UserRole) String() string {
	return string(u)
}

// IsValid checks if the user role is valid
func (u UserRole) IsValid() bool {
	switch u {
	case UserRoleCustomer, UserRoleManager, UserRoleAdmin:
		return true
	default:
		return false
	}
}

// IsAdmin checks if the role is admin or manager
func (u UserRole) IsAdmin() bool {
	return u == UserRoleAdmin || u == UserRoleManager
}

// AllUserRoles returns all valid user roles
func AllUserRoles() []UserRole {
	return []UserRole{
		UserRoleCustomer,
		UserRoleManager,
		UserRoleAdmin,
	}
}
