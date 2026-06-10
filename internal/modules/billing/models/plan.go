package models

// Plan represents a billing plan configuration (stored in config, not database).
// Monthly-only — Polar product IDs are injected from config at boot.
type Plan struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description,omitempty"`
	MonthlyID      string      `json:"monthly_id"`
	MonthlyPricing int64       `json:"monthly_pricing"`
	Features       []string    `json:"features,omitempty"`
	Recommended    bool        `json:"recommended"`
	Options        PlanOptions `json:"options"`
}

// PlanOptions represents the limits and features for a plan
type PlanOptions struct {
	MaxServers            int  `json:"max_servers"`
	MaxSitesPerServer     int  `json:"max_sites_per_server"`
	MaxDeploymentsPerSite int  `json:"max_deployments_per_site"`
	MaxTeamMembers        int  `json:"max_team_members"`
	HasBackups            bool `json:"has_backups"`
	HasMonitoring         bool `json:"has_monitoring"`
}

// PlansFromConfig returns the plan catalogue with Polar product IDs injected
// from config (monthly-only). Pricing/limits/features are fixed in code.
func PlansFromConfig(hobbyID, compactID, turboID string) []Plan {
	return []Plan{
		{
			ID:             "hobby",
			Name:           "Hobby Plan",
			MonthlyID:      hobbyID,
			MonthlyPricing: 199,
			Features: []string{
				"1 server",
				"1 site per server",
				"5 deployments retained",
				"1 team member",
			},
			Recommended: false,
			Options: PlanOptions{
				MaxServers:            1,
				MaxSitesPerServer:     1,
				MaxDeploymentsPerSite: 5,
				MaxTeamMembers:        1,
				HasBackups:            false,
				HasMonitoring:         false,
			},
		},
		{
			ID:             "compact",
			Name:           "Compact Plan",
			MonthlyID:      compactID,
			MonthlyPricing: 699,
			Features: []string{
				"3 servers",
				"10 sites per server",
				"5 deployments retained",
				"1 team member",
				"Backups",
				"Monitoring",
			},
			Recommended: true,
			Options: PlanOptions{
				MaxServers:            3,
				MaxSitesPerServer:     10,
				MaxDeploymentsPerSite: 5,
				MaxTeamMembers:        1,
				HasBackups:            true,
				HasMonitoring:         true,
			},
		},
		{
			ID:             "turbo",
			Name:           "Turbo Plan",
			MonthlyID:      turboID,
			MonthlyPricing: 2000,
			Features: []string{
				"10 servers",
				"20 sites per server",
				"5 deployments retained",
				"1 team member",
				"Backups",
				"Monitoring",
			},
			Recommended: false,
			Options: PlanOptions{
				MaxServers:            10,
				MaxSitesPerServer:     20,
				MaxDeploymentsPerSite: 5,
				MaxTeamMembers:        1,
				HasBackups:            true,
				HasMonitoring:         true,
			},
		},
	}
}

// DefaultPlans returns the plan catalogue with empty product IDs — metadata
// only (name, pricing, limits, features). Use this for read-only consumers
// (admin/overview/invitations) that don't initiate checkout. The billing
// module injects real Polar product IDs via PlansFromConfig.
func DefaultPlans() []Plan {
	return PlansFromConfig("", "", "")
}

// PlanByID returns the configured plan with the given id (e.g. "hobby") and
// true, or a zero Plan and false when no plan matches.
func PlanByID(id string) (Plan, bool) {
	for _, p := range DefaultPlans() {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}
