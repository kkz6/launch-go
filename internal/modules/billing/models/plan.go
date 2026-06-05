package models

// Plan represents a billing plan configuration (stored in config, not database)
type Plan struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description,omitempty"`
	MonthlyID      string      `json:"monthly_id"`
	YearlyID       string      `json:"yearly_id"`
	MonthlyPricing int64       `json:"monthly_pricing"`
	YearlyPricing  int64       `json:"yearly_pricing"`
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

// DefaultPlans returns default plan configurations with DodoPayments product IDs
func DefaultPlans() []Plan {
	return []Plan{
		{
			ID:             "hobby",
			Name:           "Hobby Plan",
			MonthlyID:      "pdt_0NXvMTj4rVMbbEhFF34nD",
			YearlyID:       "pdt_0NXvMZxP5iKHblFYCUQ3I",
			MonthlyPricing: 199,
			YearlyPricing:  2380,
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
			MonthlyID:      "pdt_0NXvMeSTeHZD1ETlUUuwL",
			YearlyID:       "pdt_0NXvMi7nhQ8vLsuqLMHpr",
			MonthlyPricing: 699,
			YearlyPricing:  8388,
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
			MonthlyID:      "pdt_0NXvMmccM70LXXjjCuTSd",
			YearlyID:       "pdt_0NXvMqZY4iYrFUmldDc08",
			MonthlyPricing: 2000,
			YearlyPricing:  24000,
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
