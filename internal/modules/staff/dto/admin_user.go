package dto

import "time"

// AdminTeamSubscription is the folded-in current subscription status for a team
// as shown on an admin user row. Only the fields the back-office UI needs are
// exposed; nothing billing-sensitive (customer id, card details) leaks here.
type AdminTeamSubscription struct {
	Status      string     `json:"status"`
	TrialEndsAt *time.Time `json:"trial_ends_at,omitempty"`
}

// AdminTeam is an owned team on an admin user row, with its current
// subscription folded in (nil when the team has no subscription).
type AdminTeam struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	PersonalTeam bool                   `json:"personal_team"`
	Subscription *AdminTeamSubscription `json:"subscription,omitempty"`
}

// AdminUserRow is a single row of the /admin/users listing: the user plus the
// teams they own and each team's current subscription status.
type AdminUserRow struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	StaffRole *string     `json:"staff_role,omitempty"`
	Status    string      `json:"status"`
	CreatedAt *time.Time  `json:"created_at,omitempty"`
	Teams     []AdminTeam `json:"teams"`
}
