package enumtypes

// IsValid reports whether v is one of the allowed values. Use it to collapse
// the boilerplate switch that every enum's IsValid() method otherwise needs:
//
//	// Before: 6-line switch repeated for every enum
//	func (r TeamRole) IsValid() bool {
//	    switch r {
//	    case TeamRoleOwner, TeamRoleAdmin, TeamRoleEditor, TeamRoleMember:
//	        return true
//	    }
//	    return false
//	}
//
//	// After: one line, declarative
//	func (r TeamRole) IsValid() bool {
//	    return enumtypes.IsValid(r, allTeamRoles...)
//	}
//
// Where a module already maintains an `allX` slice for AllX() accessors,
// pass it as a variadic spread (`allX...`) so the helper and the slice
// stay the single source of truth — no risk of the switch and slice
// drifting apart on a future addition.
//
// Works for any comparable type, so int-based enums get the same treatment.
func IsValid[T comparable](v T, allowed ...T) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}
