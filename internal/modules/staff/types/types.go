// Package types provides staff-authorization domain enums. Staff roles are a
// product-wide axis (who on our team can access the back-office), entirely
// separate from customer team roles in internal/modules/auth/types.
package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// StaffRole represents an internal staff member's product-wide access tier.
// A nil/absent staff role means an ordinary customer (no back-office access).
type StaffRole string

const (
	StaffRoleSupport    StaffRole = "support"
	StaffRoleSuperAdmin StaffRole = "super_admin"
)

var allStaffRoles = []StaffRole{StaffRoleSupport, StaffRoleSuperAdmin}

var staffRoleLabels = map[StaffRole]string{
	StaffRoleSupport:    "Support",
	StaffRoleSuperAdmin: "Super Admin",
}

// staffRoleHierarchy ranks tiers; higher = more access.
var staffRoleHierarchy = map[StaffRole]int{
	StaffRoleSupport:    1,
	StaffRoleSuperAdmin: 2,
}

// AllStaffRoles returns all valid staff roles.
func AllStaffRoles() []StaffRole { return allStaffRoles }

// String returns the string representation of the role.
func (r StaffRole) String() string { return string(r) }

// Label returns the human-readable label for the role.
func (r StaffRole) Label() string {
	return enumtypes.Label(r, staffRoleLabels, string(r))
}

// IsValid checks if the role is valid.
func (r StaffRole) IsValid() bool {
	return enumtypes.IsValid(r, allStaffRoles...)
}

// Level returns the hierarchy rank (super_admin:2, support:1, invalid:0).
func (r StaffRole) Level() int { return staffRoleHierarchy[r] }

// Capability helpers — call sites ask these instead of comparing strings.
func (r StaffRole) CanViewCustomerData() bool   { return r.IsValid() }
func (r StaffRole) CanImpersonate() bool        { return r.IsValid() }
func (r StaffRole) CanBypassSubscription() bool { return r.IsValid() }

// CanManageProductConfig reports whether the role may change product-wide
// configuration. Only super admins may.
func (r StaffRole) CanManageProductConfig() bool { return r == StaffRoleSuperAdmin }

// Scan implements sql.Scanner for database reads.
func (r *StaffRole) Scan(value any) error { return enumtypes.ScanString(r, value) }

// Value implements driver.Valuer for database writes.
func (r StaffRole) Value() (driver.Value, error) { return enumtypes.ValueString(r) }

// ParseStaffRole parses a string into a StaffRole.
func ParseStaffRole(s string) (StaffRole, error) {
	return enumtypes.ParseEnum(s, allStaffRoles)
}
